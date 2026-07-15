// rfcomm_darwin.m – Production IOBluetooth RFCOMM implementation (macOS)
//
// ═══════════════════════════════════════════════════════════════════════
//  Why a dedicated run-loop thread?
// ═══════════════════════════════════════════════════════════════════════
// IOBluetooth.framework delivers EVERY delegate callback through a Core
// Foundation run-loop source.  If the thread that called
// openRFCOMMChannelAsync (or writeAsync) is not actively running a
// CFRunLoop, the callback is never dispatched and the operation hangs
// forever.
//
// Go's goroutine scheduler migrates goroutines across OS threads freely,
// so we cannot rely on any goroutine's thread to keep a run loop alive.
// Instead, we create ONE permanent background pthread that runs
// CFRunLoopRun() for the process lifetime.  All IOBluetooth API calls are
// marshalled onto this thread via CFRunLoopPerformBlock, guaranteeing:
//
//   (a) Every IOBluetooth call and every callback execute on the SAME
//       thread (IOBluetooth is NOT thread-safe).
//   (b) The run loop is always active when a callback needs delivery.
//
// ═══════════════════════════════════════════════════════════════════════
//  IOBluetooth.framework limitations
// ═══════════════════════════════════════════════════════════════════════
// • The remote device MUST already be paired. Programmatic pairing
//   requires private SPI.
// • RFCOMM channel numbers must be obtained externally (SDP browse, or
//   known constant—SPP is almost always channel 1 on printers).
// • IOBluetooth is 64-bit only; not available under Rosetta 2.
// • There is no public API to enumerate bonded devices beyond
//   system_profiler / IOBluetoothDevice pairedDevices (undocumented).
// • writeAsync success means "bytes handed to the local BT stack", NOT
//   "bytes received and printed by the remote device".
// ═══════════════════════════════════════════════════════════════════════

#import <Foundation/Foundation.h>
#import <IOBluetooth/IOBluetooth.h>

#include <pthread.h>
#include <stdarg.h>
#include <stdlib.h>
#include <string.h>

#include "rfcomm_darwin.h"

// ─────────────────────────────────────────────────────────────────────────────
// Internal helpers
// ─────────────────────────────────────────────────────────────────────────────

// Forward declaration with the GCC/Clang printf-format attribute.
// NS_FORMAT_FUNCTION is for NSString format args and would cause a compile
// error here because fmt is a plain C string, not an NSString.
static void set_err(char *buf, int cap, const char *fmt, ...)
    __attribute__((format(printf, 3, 4)));

static void set_err(char *buf, int cap, const char *fmt, ...) {
    if (!buf || cap <= 0) return;
    va_list ap;
    va_start(ap, fmt);
    vsnprintf(buf, (size_t)cap, fmt, ap);
    va_end(ap);
    buf[cap - 1] = '\0';
}

// ─────────────────────────────────────────────────────────────────────────────
// Global Bluetooth worker thread
// ─────────────────────────────────────────────────────────────────────────────

static dispatch_once_t g_bt_once;
static CFRunLoopRef    g_bt_runloop = NULL; // set once; read from any thread

static void *bt_worker_main(void *arg) {
    // Transfer ownership of the startup semaphore.
    dispatch_semaphore_t ready = (__bridge_transfer dispatch_semaphore_t)arg;

    @autoreleasepool {
        // Retain the run loop reference for cross-thread use.
        g_bt_runloop = (CFRunLoopRef)CFRetain(CFRunLoopGetCurrent());

        // A CFRunLoop exits when it has no live sources.  This dummy source
        // keeps it running for the lifetime of the process.  We never remove
        // it, so the thread is intentionally parked here forever.
        CFRunLoopSourceContext ctx = {0};
        CFRunLoopSourceRef pin = CFRunLoopSourceCreate(kCFAllocatorDefault, 0, &ctx);
        CFRunLoopAddSource(g_bt_runloop, pin, kCFRunLoopDefaultMode);
        // Intentionally NOT released – this pin keeps the run loop alive.

        dispatch_semaphore_signal(ready); // unblock bt_rfcomm_init()
        CFRunLoopRun();                   // blocks until process exit

        // Defensive cleanup (not expected to be reached).
        CFRunLoopRemoveSource(g_bt_runloop, pin, kCFRunLoopDefaultMode);
        CFRelease(pin);
        CFRelease(g_bt_runloop);
        g_bt_runloop = NULL;
    }
    return NULL;
}

void bt_rfcomm_init(void) {
    dispatch_once(&g_bt_once, ^{
        dispatch_semaphore_t ready = dispatch_semaphore_create(0);

        pthread_attr_t attr;
        pthread_attr_init(&attr);
        pthread_attr_setdetachstate(&attr, PTHREAD_CREATE_DETACHED);

        pthread_t thread;
        pthread_create(&thread, &attr,
                       bt_worker_main,
                       (__bridge_retained void *)ready);
        pthread_attr_destroy(&attr);

        // Block until the worker's run loop is live and g_bt_runloop is set.
        dispatch_semaphore_wait(ready, DISPATCH_TIME_FOREVER);
    });
}

// Dispatch block synchronously onto the BT thread; wait for completion.
// MUST NOT be called from the BT thread (deadlock).
static void bt_sync(dispatch_block_t block) {
    NSCAssert(g_bt_runloop && CFRunLoopGetCurrent() != g_bt_runloop,
              @"bt_sync called from the BT thread – would deadlock");
    dispatch_semaphore_t done = dispatch_semaphore_create(0);
    CFRunLoopPerformBlock(g_bt_runloop, kCFRunLoopDefaultMode, ^{
        @autoreleasepool { block(); }
        dispatch_semaphore_signal(done);
    });
    CFRunLoopWakeUp(g_bt_runloop);
    dispatch_semaphore_wait(done, DISPATCH_TIME_FOREVER);
}

// Dispatch block asynchronously onto the BT thread (fire-and-forget).
// The block – and all its captured objects – is retained by CFRunLoop until
// executed.
static void bt_async(dispatch_block_t block) {
    CFRunLoopPerformBlock(g_bt_runloop, kCFRunLoopDefaultMode, ^{
        @autoreleasepool { block(); }
    });
    CFRunLoopWakeUp(g_bt_runloop);
}

// ─────────────────────────────────────────────────────────────────────────────
// BTRFCOMMSession – owns one RFCOMM channel and acts as its own delegate
// ─────────────────────────────────────────────────────────────────────────────
//
// Lifetime
// ────────
// A BTRFCOMMSession is created in bt_rfcomm_connect and handed to Go via
// CFBridgingRetain (bumps ARC retain count by 1).  Go holds it for the
// duration of the connection.  bt_rfcomm_close calls CFBridgingRelease, which
// transfers ownership back to an ARC local variable that deallocates when it
// goes out of scope.
//
// Thread access summary
// ─────────────────────
//   channel, pendingWriteSema, pendingWriteStatus  → BT thread only
//   isOpen, isClosed, openStatus                  → written on BT thread,
//                                                    read on Go thread (atomic)
// ─────────────────────────────────────────────────────────────────────────────

@interface BTRFCOMMSession : NSObject <IOBluetoothRFCOMMChannelDelegate>

// Owns the channel; set once during connect, nil'd on close.  BT thread only.
@property (nonatomic, strong) IOBluetoothRFCOMMChannel *channel;

// One-shot semaphore signalled by rfcommChannelOpenComplete.
@property (nonatomic) dispatch_semaphore_t openSema;

// Connection state – written on BT thread, read anywhere (atomic).
@property (atomic) IOReturn openStatus;
@property (atomic) BOOL     isOpen;
@property (atomic) BOOL     isClosed;

// Per-write synchronisation (BT thread only).
// Non-nil only while a write is in flight.  The write goroutine holds a
// local copy of the semaphore and waits on it; rfcommChannelWriteComplete
// signals it.  If a write times out the sema is cleared here; writeComplete
// then just frees the buffer and returns without signalling anyone.
@property (nonatomic) dispatch_semaphore_t pendingWriteSema;
@property (atomic)    IOReturn             pendingWriteStatus;   // written BT, read Go

@end

@implementation BTRFCOMMSession

- (instancetype)init {
    if (!(self = [super init])) return nil;
    _openSema   = dispatch_semaphore_create(0);
    _openStatus = kIOReturnError;
    return self;
}

// ── Delegate: channel fully opened ──────────────────────────────────────────

- (void)rfcommChannelOpenComplete:(IOBluetoothRFCOMMChannel *)rfcommChannel
                           status:(IOReturn)error {
    // Guard: if we already timed out, don't write into freed state.
    // We still signal the semaphore so that if the caller is somehow still
    // blocked the semaphore is correct.
    self.openStatus = error;
    self.isOpen     = (error == kIOReturnSuccess);
    dispatch_semaphore_signal(self.openSema);
}

// ── Delegate: channel closed (remote disconnect or local close) ──────────────

- (void)rfcommChannelClosed:(IOBluetoothRFCOMMChannel *)rfcommChannel {
    self.isOpen   = NO;
    self.isClosed = YES;

    // Unblock any write waiting for its completion callback so the Go
    // goroutine doesn't hang until its per-write timeout fires.
    dispatch_semaphore_t ws = self.pendingWriteSema;
    if (ws) {
        self.pendingWriteStatus = kIOReturnNotOpen;
        self.pendingWriteSema   = nil;
        dispatch_semaphore_signal(ws);
    }
}

// ── Delegate: async write acknowledged by local BT stack ────────────────────

- (void)rfcommChannelWriteComplete:(IOBluetoothRFCOMMChannel *)rfcommChannel
                            refcon:(void *)refcon
                            status:(IOReturn)error {
    // refcon is the malloc'd write buffer; ALWAYS free it, even if the
    // write was abandoned due to a timeout on the Go side.
    free(refcon);

    dispatch_semaphore_t ws = self.pendingWriteSema;
    if (ws) {
        self.pendingWriteStatus = error;
        self.pendingWriteSema   = nil;
        dispatch_semaphore_signal(ws);
    }
    // If ws is nil the write timed out; the Go goroutine no longer waits.
    // buf has been freed above; nothing else to do.
}

// ── Delegate: incoming data (printers are write-only; discard silently) ──────

- (void)rfcommChannelData:(IOBluetoothRFCOMMChannel *)rfcommChannel
                     data:(void *)dataPointer
                   length:(size_t)dataLength {
    (void)rfcommChannel;
    (void)dataPointer;
    (void)dataLength;
}

@end

// ─────────────────────────────────────────────────────────────────────────────
// bt_rfcomm_connect
// ─────────────────────────────────────────────────────────────────────────────

BTRFCOMMHandle bt_rfcomm_connect(const char *mac, uint8_t rfchannel,
                                  int timeout_ms,
                                  char *err_buf, int err_cap) {
    bt_rfcomm_init();

    if (timeout_ms <= 0) timeout_ms = 10000;

    // ── Normalise MAC to IOBluetooth's preferred "AA-BB-CC-DD-EE-FF" form ───
    if (!mac || strlen(mac) != 17) {
        set_err(err_buf, err_cap,
                "invalid MAC address %s (expected 17-char AA:BB:CC:DD:EE:FF)",
                mac ? mac : "(null)");
        return NULL;
    }
    char norm[18];
    for (int i = 0; i < 17; i++) {
        if (i % 3 == 2) {
            norm[i] = '-';
        } else {
            char c = mac[i];
            // Upper-case hex digits; leave separator positions unchanged.
            norm[i] = (c >= 'a' && c <= 'f') ? (char)(c - ('a' - 'A')) : c;
        }
    }
    norm[17] = '\0';

    NSString *macStr = [NSString stringWithUTF8String:norm];

    // ── Create session ───────────────────────────────────────────────────────
    BTRFCOMMSession *session = [[BTRFCOMMSession alloc] init];

    // ── Issue openRFCOMMChannelAsync on the BT thread ────────────────────────
    //
    // We use the async variant so that openRFCOMMChannelAsync returns
    // immediately, the BT thread re-enters CFRunLoopRun, and the connection
    // callbacks can be delivered.  (The sync variant would block the BT
    // thread inside its own internal mini run loop, preventing other blocks
    // queued via CFRunLoopPerformBlock from executing concurrently.)
    __block BOOL started = NO;

    // Capture macStr (an NSString object pointer) rather than `norm` (a C
    // array).  Clang blocks cannot capture C array types by value; capturing
    // an NSString pointer is safe and carries identical information.
    bt_sync(^{
        IOBluetoothDevice *device = [IOBluetoothDevice deviceWithAddressString:macStr];
        if (!device) {
            set_err(err_buf, err_cap,
                    "IOBluetoothDevice not found for %s — "
                    "pair the device first in System Preferences → Bluetooth",
                    macStr.UTF8String);
            // Signal the sema so the caller is not left waiting.
            dispatch_semaphore_signal(session.openSema);
            return;
        }

        IOBluetoothRFCOMMChannel *ch = nil;
        IOReturn ret = [device openRFCOMMChannelAsync:&ch
                                        withChannelID:rfchannel
                                             delegate:session];
        if (ret != kIOReturnSuccess || ch == nil) {
            set_err(err_buf, err_cap,
                    "openRFCOMMChannelAsync failed for %s ch %u: IOReturn=0x%08x",
                    macStr.UTF8String, (unsigned)rfchannel, (unsigned)ret);
            dispatch_semaphore_signal(session.openSema);
            return;
        }

        // Strong-retain the channel so it stays alive until we explicitly
        // release it.  IOBluetooth does NOT retain the delegate, so as long
        // as session holds the channel and we hold session, both live.
        session.channel = ch;
        started = YES;
        // rfcommChannelOpenComplete will signal session.openSema.
    });

    if (!started) {
        // Error already written to err_buf by the block above.
        return NULL;
    }

    // ── Wait for rfcommChannelOpenComplete ───────────────────────────────────
    dispatch_time_t deadline =
        dispatch_time(DISPATCH_TIME_NOW, (int64_t)timeout_ms * NSEC_PER_MSEC);

    if (dispatch_semaphore_wait(session.openSema, deadline) != 0) {
        // Timeout: clean up on the BT thread so no future callback touches
        // a session whose Go-side wrapper has already been discarded.
        // We capture session strongly so it stays alive until the async
        // block executes, then ARC releases it.
        BTRFCOMMSession *s = session;
        bt_async(^{
            if (!s.isClosed) {
                s.isClosed           = YES;
                s.channel.delegate   = nil; // prevent further callbacks
                [s.channel closeChannel];
                s.channel            = nil;
            }
        });
        set_err(err_buf, err_cap,
                "RFCOMM open timed out after %d ms for %s ch %u "
                "(device may be out of range or busy)",
                timeout_ms, macStr.UTF8String, (unsigned)rfchannel);
        return NULL; // session ARC-released when this scope returns
    }

    if (!session.isOpen) {
        // rfcommChannelOpenComplete delivered an error status.
        BTRFCOMMSession *s = session;
        bt_sync(^{
            s.isClosed         = YES;
            s.channel.delegate = nil;
            [s.channel closeChannel];
            s.channel          = nil;
        });
        set_err(err_buf, err_cap,
                "RFCOMM open failed for %s ch %u: IOReturn=0x%08x",
                macStr.UTF8String, (unsigned)rfchannel, (unsigned)session.openStatus);
        return NULL;
    }

    // ── Success: transfer ownership to Go ────────────────────────────────────
    // CFBridgingRetain bumps the ARC retain count by 1.  The caller owns this
    // reference until bt_rfcomm_close calls CFBridgingRelease.
    return (BTRFCOMMHandle)CFBridgingRetain(session);
}

// ─────────────────────────────────────────────────────────────────────────────
// bt_rfcomm_write
// ─────────────────────────────────────────────────────────────────────────────

int bt_rfcomm_write(BTRFCOMMHandle handle,
                    const uint8_t *data, int len,
                    int timeout_ms,
                    char *err_buf, int err_cap) {
    if (!handle) { set_err(err_buf, err_cap, "NULL handle");   return -1; }
    if (!data)   { set_err(err_buf, err_cap, "NULL data buf"); return -1; }
    if (len <= 0) return 0;
    if (timeout_ms <= 0) timeout_ms = 5000;

    BTRFCOMMSession *session = (__bridge BTRFCOMMSession *)handle;

    if (session.isClosed || !session.isOpen) {
        set_err(err_buf, err_cap, "connection is not open");
        return -1;
    }

    // ── Fetch negotiated MTU (BT thread) ────────────────────────────────────
    // getMTU is safe to call only on the BT thread.
    __block BluetoothRFCOMMMTU mtu = 0;
    bt_sync(^{ mtu = [session.channel getMTU]; });
    if (mtu < 1) mtu = 127; // conservative safe fallback

    int written = 0;

    while (written < len) {
        // ── Quick guard before each chunk ────────────────────────────────────
        if (session.isClosed) {
            set_err(err_buf, err_cap, "connection closed mid-write at byte %d", written);
            return -1;
        }

        int chunk = len - written;
        if (chunk > (int)mtu) chunk = (int)mtu;

        // ── Allocate write buffer ────────────────────────────────────────────
        // This buffer is passed as refcon to writeAsync and ALWAYS freed in
        // rfcommChannelWriteComplete, even if the Go side has timed out and
        // abandoned the write.  Do NOT free it here on any code path after a
        // successful writeAsync call.
        void *buf = malloc((size_t)chunk);
        if (!buf) {
            set_err(err_buf, err_cap, "malloc(%d) failed", chunk);
            return -1;
        }
        memcpy(buf, data + written, (size_t)chunk);

        // ── Create a per-chunk write semaphore ───────────────────────────────
        // Using a fresh semaphore per chunk avoids any cross-chunk signalling
        // ambiguity (e.g. a delayed writeComplete from a previous timed-out
        // chunk signalling the wrong waiter).
        dispatch_semaphore_t ws = dispatch_semaphore_create(0);

        // ── Dispatch writeAsync to the BT thread ─────────────────────────────
        __block IOReturn write_ret = kIOReturnSuccess;

        bt_sync(^{
            if (session.isClosed || !session.isOpen) {
                // Connection dropped between the guard above and now.
                write_ret = kIOReturnNotOpen;
                free(buf); // never handed to IOBluetooth; our responsibility
                return;
            }

            // Register the semaphore BEFORE calling writeAsync so that
            // rfcommChannelWriteComplete can never fire between the writeAsync
            // call and the registration.
            session.pendingWriteSema   = ws;
            session.pendingWriteStatus = kIOReturnError;

            IOReturn ret = [session.channel writeAsync:buf
                                               length:(UInt16)chunk
                                               refcon:buf];
            if (ret != kIOReturnSuccess) {
                // writeAsync failed synchronously; IOBluetooth will NOT call
                // writeComplete, so we must free buf ourselves.
                session.pendingWriteSema = nil;
                free(buf);
                write_ret = ret;
            }
            // On success: buf is owned by IOBluetooth until writeComplete fires.
        });

        if (write_ret != kIOReturnSuccess) {
            set_err(err_buf, err_cap,
                    "writeAsync failed at byte %d: IOReturn=0x%08x",
                    written, (unsigned)write_ret);
            return -1;
        }

        // ── Wait for rfcommChannelWriteComplete ──────────────────────────────
        dispatch_time_t deadline =
            dispatch_time(DISPATCH_TIME_NOW, (int64_t)timeout_ms * NSEC_PER_MSEC);

        if (dispatch_semaphore_wait(ws, deadline) != 0) {
            // Write timed out.  Clear pendingWriteSema on the BT thread so
            // rfcommChannelWriteComplete (when it eventually fires) sees nil
            // and knows not to signal an abandoned semaphore.  The write
            // buffer will be freed by writeComplete regardless.
            bt_sync(^{
                if (session.pendingWriteSema == ws) {
                    session.pendingWriteSema = nil;
                }
                // If ws was already replaced (e.g. writeComplete fired and was
                // processed between our timeout and this bt_sync), the check
                // above is a no-op and buf was already freed by writeComplete.
            });
            set_err(err_buf, err_cap,
                    "write timed out after %d ms at byte %d",
                    timeout_ms, written);
            return -1;
        }

        // ── Check write completion status ────────────────────────────────────
        IOReturn ws_status = session.pendingWriteStatus; // atomic read
        if (ws_status != kIOReturnSuccess) {
            set_err(err_buf, err_cap,
                    "write failed at byte %d: IOReturn=0x%08x",
                    written, (unsigned)ws_status);
            return -1;
        }

        written += chunk;
    }

    return written;
}

// ─────────────────────────────────────────────────────────────────────────────
// bt_rfcomm_close
// ─────────────────────────────────────────────────────────────────────────────

void bt_rfcomm_close(BTRFCOMMHandle handle) {
    if (!handle) return;

    // CFBridgingRelease decrements the retain count by 1 and transfers
    // ownership to the local ARC variable.  When 'session' goes out of
    // scope at the end of this function, ARC releases it.
    BTRFCOMMSession *session = (BTRFCOMMSession *)CFBridgingRelease(handle);

    if (session.isClosed) return; // already closed (e.g. remote disconnect)

    bt_sync(^{
        session.isClosed = YES;

        // If a write goroutine is waiting, unblock it immediately so it
        // doesn't spin until its per-chunk timeout fires.
        dispatch_semaphore_t ws = session.pendingWriteSema;
        if (ws) {
            session.pendingWriteStatus = kIOReturnAborted;
            session.pendingWriteSema   = nil;
            dispatch_semaphore_signal(ws);
        }

        // Setting delegate = nil BEFORE closeChannel ensures that any
        // rfcommChannelClosed callback triggered by closeChannel is not
        // delivered to an object that may be mid-dealloc.
        session.channel.delegate = nil;
        [session.channel closeChannel];
        session.channel = nil;
    });

    // 'session' ARC-releases here.  Since channel is nil, the session has no
    // outstanding strong references to IOBluetooth objects.
}
