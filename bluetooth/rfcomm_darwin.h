// rfcomm_darwin.h – C interface to the IOBluetooth RFCOMM implementation.
//
// This header is included by rfcomm_darwin.go via cgo. It is a plain-C header
// so that it can be processed by both the C compiler (for the .m translation
// unit) and the cgo preprocessor.
//
// Build requirements
// ──────────────────
//   • macOS 10.13+ (IOBluetooth is available since 10.2, but 10.13+ is tested)
//   • Link: -framework IOBluetooth -framework Foundation -framework CoreFoundation
//   • Compile the .m file with ARC: -fobjc-arc
//
// Usage
// ─────
//   The device MUST already be paired in System Preferences → Bluetooth.
//   Programmatic pairing requires private APIs and is intentionally unsupported.
//
//   1. Call bt_rfcomm_connect() to open an RFCOMM channel.
//   2. Call bt_rfcomm_write() to send raw ESC/POS bytes.
//   3. Call bt_rfcomm_close() to release all resources.
//
// Thread safety
// ─────────────
//   All functions are safe to call concurrently from different threads.
//   Each BTRFCOMMHandle is an independent connection; handles must not be
//   shared across goroutines without external synchronisation.

#pragma once

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

// Opaque handle to one open RFCOMM connection.
// Internally this is a retained Objective-C BTRFCOMMSession pointer.
typedef void *BTRFCOMMHandle;

// bt_rfcomm_init starts the internal Bluetooth worker thread (a pthread that
// runs CFRunLoopRun for the lifetime of the process). Idempotent – safe to
// call multiple times; called automatically by bt_rfcomm_connect.
void bt_rfcomm_init(void);

// bt_rfcomm_connect opens an RFCOMM channel to the already-paired device.
//
//   mac        – Bluetooth address: "AA:BB:CC:DD:EE:FF" or
//                "AA-BB-CC-DD-EE-FF" (upper/lower case accepted).
//   rfchannel  – RFCOMM channel number 1–30.  Most SPP printers use channel 1.
//   timeout_ms – open timeout in milliseconds; 0 → 10 000 ms default.
//   err_buf    – caller-supplied buffer; filled with a NUL-terminated error
//                string on failure.  Must not be NULL.
//   err_cap    – capacity of err_buf in bytes; must be > 0.
//
// Returns a non-NULL handle on success, NULL on failure (error in err_buf).
BTRFCOMMHandle bt_rfcomm_connect(const char *mac, uint8_t rfchannel,
                                  int timeout_ms,
                                  char *err_buf, int err_cap);

// bt_rfcomm_write sends len bytes of data over the open RFCOMM channel.
//
// Data is automatically fragmented at the channel's negotiated MTU so the
// caller may pass arbitrarily large payloads.  The function blocks until every
// byte has been acknowledged by the local Bluetooth stack (not necessarily by
// the printer's application layer).
//
//   timeout_ms – per-MTU-chunk write timeout; 0 → 5 000 ms default.
//
// Returns the total number of bytes written (== len) on success, or -1 on
// error (error in err_buf).
int bt_rfcomm_write(BTRFCOMMHandle handle,
                    const uint8_t *data, int len,
                    int timeout_ms,
                    char *err_buf, int err_cap);

// bt_rfcomm_close closes the RFCOMM channel and releases all resources.
// After this call the handle must not be used again.
// Safe to call from any thread; no-op if already closed.
void bt_rfcomm_close(BTRFCOMMHandle handle);

#ifdef __cplusplus
}
#endif
