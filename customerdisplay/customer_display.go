package customerdisplay

import "sync"

// MonitorInfo describes a connected display.
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

// RegisterMonitorChangeCallback registers a function called when the monitor
// configuration changes (monitor added or removed).
func RegisterMonitorChangeCallback(cb func()) {
	callbackMu.Lock()
	defer callbackMu.Unlock()
	onMonitorChangeCallback = cb
}

func triggerCallback() {
	callbackMu.Lock()
	cb := onMonitorChangeCallback
	callbackMu.Unlock()
	if cb != nil {
		cb()
	}
}

// Init sets up platform-level monitor-change notifications.
func Init() {
	platformInit()
}

// GetMonitors returns all connected monitors.
func GetMonitors() []MonitorInfo {
	return platformGetMonitors()
}

// Open opens (or repositions) the customer display on the given monitor and
// loads the provided URL.
func Open(monitorID, url string) {
	platformOpen(monitorID, url)
}

// Close closes the customer display window.
func Close() {
	platformClose()
}

// Reload reloads the current page in the customer display.
func Reload() {
	platformReload()
}

// Navigate loads a new URL in the customer display.
func Navigate(url string) {
	platformNavigate(url)
}

// Identify flashes a numbered overlay on every connected monitor for ~3 s.
func Identify() {
	platformIdentify()
}

// Test shows a brief test screen on the specified monitor for ~3 s.
func Test(monitorID string) {
	platformTest(monitorID)
}
