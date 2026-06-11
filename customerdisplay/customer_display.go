package customerdisplay

/*
#cgo pkg-config: gtk+-3.0 webkit2gtk-4.1
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
	"sync"
	"unsafe"
)

type MonitorInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	X         int    `json:"x"`
	Y         int    `json:"y"`
	IsPrimary bool   `json:"isPrimary"`
}

var (
	onMonitorChangeCallback func()
	callbackMu              sync.Mutex
)

func RegisterMonitorChangeCallback(cb func()) {
	callbackMu.Lock()
	defer callbackMu.Unlock()
	onMonitorChangeCallback = cb
}

//export goOnMonitorAdded
func goOnMonitorAdded() {
	triggerCallback()
}

//export goOnMonitorRemoved
func goOnMonitorRemoved() {
	triggerCallback()
}

func triggerCallback() {
	callbackMu.Lock()
	cb := onMonitorChangeCallback
	callbackMu.Unlock()
	if cb != nil {
		cb()
	}
}

func Init() {
	C.setup_monitor_signals_c()
}

func GetMonitors() []MonitorInfo {
	jsonC := C.get_monitors_json_c()
	defer C.free(unsafe.Pointer(jsonC))
	
	var monitors []MonitorInfo
	if err := json.Unmarshal([]byte(C.GoString(jsonC)), &monitors); err != nil {
		return []MonitorInfo{}
	}
	return monitors
}

func Open(monitorID, url string) {
	cID := C.CString(monitorID)
	cURL := C.CString(url)
	defer C.free(unsafe.Pointer(cID))
	defer C.free(unsafe.Pointer(cURL))
	C.open_customer_display_c(cID, cURL)
}

func Close() {
	C.close_customer_display_c()
}

func Reload() {
	C.reload_customer_display_c()
}

func Navigate(url string) {
	cURL := C.CString(url)
	defer C.free(unsafe.Pointer(cURL))
	C.navigate_customer_display_c(cURL)
}

func Identify() {
	C.identify_monitors_c()
}

func Test(monitorID string) {
	cID := C.CString(monitorID)
	defer C.free(unsafe.Pointer(cID))
	C.open_test_display_c(cID)
}
