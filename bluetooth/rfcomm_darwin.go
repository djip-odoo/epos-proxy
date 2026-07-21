//go:build darwin && cgo

// Package bluetooth provides Bluetooth Classic (RFCOMM/SPP) connectivity for
// paired thermal printers on macOS.
//
// # Native IOBluetooth RFCOMM
//
// This file implements a pure IOBluetooth.framework RFCOMM path that bypasses
// the /dev/cu.* serial-port layer entirely.  It uses cgo to bridge into
// Objective-C, where a single dedicated pthread runs a CFRunLoop for the
// process lifetime, hosting all IOBluetooth API calls and delegate callbacks.
//
// # Build Requirements
//
//   - macOS 10.13+
//   - Xcode Command Line Tools (clang with Objective-C support)
//   - No special entitlements are required; access to IOBluetooth.framework
//     is permitted for regular user processes.
//   - Build with: GOFLAGS= go build ./... (standard go build)
//
// # Entitlements
//
// No sandbox entitlements are required for IOBluetooth RFCOMM on macOS when
// the binary is not sandboxed.  If the app is sandboxed (App Store / Gatekeeper
// hardened runtime), add the Bluetooth entitlement:
//
//	<key>com.apple.security.device.bluetooth</key><true/>
//
// # Example – sending ESC/POS to a paired SPP printer
//
//	conn, err := bluetooth.Connect("AA:BB:CC:DD:EE:FF", 1)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer conn.Close()
//
//	job := []byte{
//	    0x1B, '@',                    // ESC @ – initialise printer
//	    'H', 'e', 'l', 'l', 'o', '\n',
//	    '\n', '\n',                   // feed
//	}
//	if err := conn.Write(job); err != nil {
//	    log.Fatal(err)
//	}
package bluetooth

// ─── cgo directives ──────────────────────────────────────────────────────────
// The Go build system compiles rfcomm_darwin.m because its name ends in
// _darwin (OS-specific file-name constraint) and cgo knows to treat .m files
// as Objective-C.
//
// -fobjc-arc:  Enable Automatic Reference Counting in the .m file so that
//              ObjC objects and GCD types are memory-managed automatically.
//
// Frameworks:
//   IOBluetooth  – RFCOMM channel API (IOBluetoothDevice, IOBluetoothRFCOMMChannel)
//   Foundation   – NSObject, NSString, autorelease pools
//   CoreFoundation – CFRunLoop, CFRunLoopPerformBlock
//
// Note: the CFLAGS apply to every C/ObjC file in this package; -fobjc-arc is
// a no-op on plain C files and harmless for any existing .c files.

// #cgo CFLAGS: -fobjc-arc
// #cgo LDFLAGS: -framework IOBluetooth -framework Foundation -framework CoreFoundation
// #include "rfcomm_darwin.h"
// #include <stdlib.h>
import "C"

import (
	"fmt"
	"io"
	"net"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"epos-proxy/logger"
	"epos-proxy/util"
)

const rfcommErrBufSize = 512

// ─────────────────────────────────────────────────────────────────────────────
// Connection
// ─────────────────────────────────────────────────────────────────────────────

// Connection represents an open Bluetooth Classic (RFCOMM/SPP) connection to a
// thermal printer.
//
// All methods are safe to call from multiple goroutines concurrently.
// Writes are serialised internally; the caller need not add extra locking.
//
// A Connection must be released with [Connection.Close] when done.  If the
// caller forgets, a runtime finalizer will close the underlying channel and
// log a warning, but relying on the finalizer for timely resource release is
// strongly discouraged.
type Connection struct {
	handle C.BTRFCOMMHandle // retained ObjC BTRFCOMMSession*; nil after Close
	mu     sync.Mutex       // serialises Write calls and protects closed
	closed bool
	mac    string // for RemoteAddr()
}

// rfcommAddr is a minimal net.Addr for RFCOMM connections.
type rfcommAddr struct{ addr string }

func (a rfcommAddr) Network() string { return "rfcomm" }
func (a rfcommAddr) String() string  { return a.addr }

// ─────────────────────────────────────────────────────────────────────────────
// Connect
// ─────────────────────────────────────────────────────────────────────────────

// Connect opens an RFCOMM channel to the Bluetooth Classic device at mac on
// channel number rfchannel and returns a ready-to-write [Connection].
//
// The device must already be paired in System Preferences → Bluetooth.
// Programmatic pairing is not supported.
//
//   - mac       – Bluetooth MAC address, e.g. "AA:BB:CC:DD:EE:FF" (colon or
//     hyphen separators, upper or lower case).
//   - rfchannel – RFCOMM channel number (1–30).  SPP thermal printers almost
//     always use channel 1; consult the device's SDP record if
//     unsure.
//
// Connect blocks until the channel is fully open or the 10-second default
// timeout expires.
func Connect(mac string, rfchannel uint8) (*Connection, error) {
	mac = util.NormalizeAddress(mac)
	if err := util.ValidateAddress(mac); err != nil {
		return nil, fmt.Errorf("bluetooth/rfcomm: invalid MAC address %q: %w", mac, err)
	}

	cmac := C.CString(mac)
	defer C.free(unsafe.Pointer(cmac))

	errBuf := (*C.char)(C.malloc(rfcommErrBufSize))
	if errBuf == nil {
		return nil, fmt.Errorf("bluetooth/rfcomm: malloc failed for error buffer")
	}
	defer C.free(unsafe.Pointer(errBuf))
	*errBuf = 0

	logger.Infof("BT/darwin/rfcomm: connecting to %s ch %d via IOBluetooth", mac, rfchannel)

	handle := C.bt_rfcomm_connect(cmac, C.uint8_t(rfchannel), 0, errBuf, rfcommErrBufSize)
	if handle == nil {
		return nil, fmt.Errorf("bluetooth/rfcomm: %s", C.GoString(errBuf))
	}

	conn := &Connection{handle: handle, mac: mac}

	// Finalizer as a safety net only: callers must call Close() explicitly.
	runtime.SetFinalizer(conn, func(c *Connection) {
		if !c.closed {
			logger.Warnf("BT/darwin/rfcomm: Connection for %s garbage-collected without Close()", mac)
			c.closeInternal()
		}
	})

	logger.Infof("BT/darwin/rfcomm: connected to %s ch %d", mac, rfchannel)
	return conn, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Write
// ─────────────────────────────────────────────────────────────────────────────

// Read implements net.Conn. RFCOMM printers are write-only from the host side;
// Read immediately returns io.EOF so that callers using net.Conn abstractions
// (e.g. io.Copy) terminate cleanly without spinning.
func (c *Connection) Read(_ []byte) (int, error) {
	return 0, io.EOF
}

// Write sends data to the printer over the RFCOMM channel.
//
// Write is safe to call from multiple goroutines: concurrent calls are
// serialised internally.  The function blocks until all bytes have been
// handed off to the local Bluetooth stack (not necessarily received by the
// printer application) or an error occurs.
//
// Data is automatically fragmented at the channel's negotiated MTU (typically
// 672 bytes for Bluetooth 2.0+ EDR), so callers may pass arbitrarily large
// payloads such as full ESC/POS print jobs in one call.
func (c *Connection) Write(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return 0, fmt.Errorf("bluetooth/rfcomm: write on closed connection")
	}

	errBuf := (*C.char)(C.malloc(rfcommErrBufSize))
	if errBuf == nil {
		return 0, fmt.Errorf("bluetooth/rfcomm: malloc failed for error buffer")
	}
	defer C.free(unsafe.Pointer(errBuf))
	*errBuf = 0

	// Pass a pointer to the Go slice backing array.  cgo pins Go memory for
	// the duration of C calls; bt_rfcomm_write copies each MTU chunk into a
	// malloc'd buffer before calling writeAsync, so the Go slice may be
	// released once bt_rfcomm_write returns.
	ret := C.bt_rfcomm_write(
		c.handle,
		(*C.uint8_t)(unsafe.Pointer(&data[0])),
		C.int(len(data)),
		0, // use default timeout (5 000 ms per chunk)
		errBuf, rfcommErrBufSize,
	)
	if ret < 0 {
		return 0, fmt.Errorf("bluetooth/rfcomm: %s", C.GoString(errBuf))
	}
	return int(ret), nil
}

// LocalAddr implements net.Conn.
func (c *Connection) LocalAddr() net.Addr { return rfcommAddr{"local"} }

// RemoteAddr implements net.Conn.
func (c *Connection) RemoteAddr() net.Addr { return rfcommAddr{c.mac} }

// SetDeadline implements net.Conn. IOBluetooth write timeouts are configured
// per-operation (defaulting to 5 s); this no-op satisfies the interface.
func (c *Connection) SetDeadline(_ time.Time) error { return nil }

// SetReadDeadline implements net.Conn.
func (c *Connection) SetReadDeadline(_ time.Time) error { return nil }

// SetWriteDeadline implements net.Conn.
func (c *Connection) SetWriteDeadline(_ time.Time) error { return nil }

// ─────────────────────────────────────────────────────────────────────────────
// Close
// ─────────────────────────────────────────────────────────────────────────────

// Close closes the RFCOMM channel and releases all underlying resources.
//
// Close is safe to call from any goroutine and from multiple goroutines
// concurrently.  Subsequent calls are no-ops.  Any goroutine blocked in
// [Connection.Write] will be unblocked with an error.
func (c *Connection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}
	runtime.SetFinalizer(c, nil) // disarm the safety-net finalizer
	return c.closeInternal()
}

// closeInternal performs the actual close without holding mu.
// Callers must ensure mu is held (or that no concurrent access is possible,
// as in the finalizer).
func (c *Connection) closeInternal() error {
	c.closed = true
	C.bt_rfcomm_close(c.handle)
	c.handle = nil
	return nil
}
