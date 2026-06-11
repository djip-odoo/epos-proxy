//go:build darwin

package customerdisplay

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework WebKit -framework AppKit -framework Foundation

#include <stdlib.h>

char* get_monitors_json_c();
void open_customer_display_c(const char* monitor_id, const char* url);
void close_customer_display_c();
void reload_customer_display_c();
void navigate_customer_display_c(const char* url);
void identify_monitors_c();
void open_test_display_c(const char* monitor_id);
void setup_monitor_signals_c();
*/
import "C"

import (
	"encoding/json"
	"unsafe"
)

// ── Monitor-change callbacks (called back from Objective-C) ───────────────

//export goOnMonitorAdded
func goOnMonitorAdded() {
	triggerCallback()
}

//export goOnMonitorRemoved
func goOnMonitorRemoved() {
	triggerCallback()
}

// ── Platform implementation ───────────────────────────────────────────────

func platformInit() {
	C.setup_monitor_signals_c()
}

func platformGetMonitors() []MonitorInfo {
	jsonC := C.get_monitors_json_c()
	defer C.free(unsafe.Pointer(jsonC))

	var monitors []MonitorInfo
	if err := json.Unmarshal([]byte(C.GoString(jsonC)), &monitors); err != nil {
		return []MonitorInfo{}
	}
	return monitors
}

func platformOpen(monitorID, url string) {
	cID := C.CString(monitorID)
	cURL := C.CString(url)
	defer C.free(unsafe.Pointer(cID))
	defer C.free(unsafe.Pointer(cURL))
	C.open_customer_display_c(cID, cURL)
}

func platformClose() {
	C.close_customer_display_c()
}

func platformReload() {
	C.reload_customer_display_c()
}

func platformNavigate(url string) {
	cURL := C.CString(url)
	defer C.free(unsafe.Pointer(cURL))
	C.navigate_customer_display_c(cURL)
}

func platformIdentify() {
	C.identify_monitors_c()
}

func platformTest(monitorID string) {
	cID := C.CString(monitorID)
	defer C.free(unsafe.Pointer(cID))
	C.open_test_display_c(cID)
}
