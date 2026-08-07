//go:build windows

// Package customerdisplay provides the Windows customer display backend.
//
// Implementation uses:
//   - golang.org/x/sys/windows for Win32 monitor enumeration
//   - github.com/wailsapp/go-webview2 for WebView2 rendering
//
// Each public function maps to the platform-agnostic shim in customer_display.go.
package customerdisplay

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
	"unsafe"

	"github.com/wailsapp/go-webview2/pkg/edge"
	"golang.org/x/sys/windows"
)

// ── Win32 types & constants ───────────────────────────────────────────────

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	shcore   = windows.NewLazySystemDLL("shcore.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procEnumDisplayMonitors = user32.NewProc("EnumDisplayMonitors")
	procGetMonitorInfo      = user32.NewProc("GetMonitorInfoW")
	procSetWindowPos        = user32.NewProc("SetWindowPos")
	procShowWindow          = user32.NewProc("ShowWindow")
	procGetSystemMetrics    = user32.NewProc("GetSystemMetrics")
	procGetPrimaryMonitor   = user32.NewProc("MonitorFromPoint")
	procMessageBox          = user32.NewProc("MessageBoxW")
	procSetProcessDPIAware  = user32.NewProc("SetProcessDPIAware")
)

type POINT struct{ X, Y int32 }
type RECT struct{ Left, Top, Right, Bottom int32 }

type MONITORINFOEX struct {
	CbSize    uint32
	RcMonitor RECT
	RcWork    RECT
	DwFlags   uint32
	SzDevice  [32]uint16
}

const (
	MONITOR_DEFAULTTOPRIMARY = 1
	MONITOR_DEFAULTTONEAREST = 2
	MONITORINFOF_PRIMARY     = 1
	SWP_SHOWWINDOW           = 0x0040
	SWP_NOACTIVATE           = 0x0010
	SW_SHOW                  = 5
	SW_HIDE                  = 0
)

// ── Singleton state ───────────────────────────────────────────────────────

type displayState struct {
	mu         sync.Mutex
	wv         *edge.Chromium // WebView2 controller
	hwnd       uintptr        // host HWND
	currentURL string
}

var state displayState

// ── Monitor enumeration (Win32 EnumDisplayMonitors) ───────────────────────

type monitorEnumData struct {
	monitors []MonitorInfo
}

func enumMonitorsCallback(hMonitor, hdcMonitor uintptr, lprcMonitor *RECT, dwData uintptr) uintptr {
	data := (*monitorEnumData)(unsafe.Pointer(dwData))

	var info MONITORINFOEX
	info.CbSize = uint32(unsafe.Sizeof(info))
	procGetMonitorInfo.Call(hMonitor, uintptr(unsafe.Pointer(&info)))

	name := windows.UTF16ToString(info.SzDevice[:])
	isPrimary := (info.DwFlags & MONITORINFOF_PRIMARY) != 0

	idx := len(data.monitors)
	id := fmt.Sprintf("Win-%s-%dx%d-%d-%d-%d",
		name,
		info.RcMonitor.Right-info.RcMonitor.Left,
		info.RcMonitor.Bottom-info.RcMonitor.Top,
		info.RcMonitor.Left,
		info.RcMonitor.Top,
		idx,
	)

	data.monitors = append(data.monitors, MonitorInfo{
		ID:        id,
		Name:      name,
		Width:     int(info.RcMonitor.Right - info.RcMonitor.Left),
		Height:    int(info.RcMonitor.Bottom - info.RcMonitor.Top),
		X:         int(info.RcMonitor.Left),
		Y:         int(info.RcMonitor.Top),
		IsPrimary: isPrimary,
	})
	return 1 // continue enumeration
}

func enumerateMonitors() []MonitorInfo {
	data := &monitorEnumData{}
	cb := windows.NewCallback(func(hMonitor, hdcMonitor uintptr, lprcMonitor *RECT, dwData uintptr) uintptr {
		return enumMonitorsCallback(hMonitor, hdcMonitor, lprcMonitor, dwData)
	})
	procEnumDisplayMonitors.Call(0, 0, cb, uintptr(unsafe.Pointer(data)))
	return data.monitors
}

func findMonitor(monitorID string) *MonitorInfo {
	for _, m := range enumerateMonitors() {
		if m.ID == monitorID {
			mc := m
			return &mc
		}
	}
	return nil
}

// ── WndProc for the WebView2 host window ─────────────────────────────────

var (
	wndProcCallback uintptr
	hostClass       = windows.StringToUTF16Ptr("CDWebViewHost")
)

func wndProc(hwnd, msg, wp, lp uintptr) uintptr {
	const WM_DESTROY = 0x0002
	const WM_SIZE = 0x0005
	if msg == WM_DESTROY {
		state.mu.Lock()
		state.hwnd = 0
		state.wv = nil
		state.currentURL = ""
		state.mu.Unlock()
		return 0
	}
	if msg == WM_SIZE && state.wv != nil {
		// Resize the WebView2 controller to fill the new window size
		// go-webview2 exposes Resize on the Chromium object
		// state.wv.Resize() // called below in openOnMonitor
		_ = lp
	}
	ret, _, _ := user32.NewProc("DefWindowProcW").Call(hwnd, msg, wp, lp)
	return ret
}

// registerWindowClass registers our host HWND class (once).
var registerOnce sync.Once

func registerClass(hInstance uintptr) {
	registerOnce.Do(func() {
		type WNDCLASSEX struct {
			CbSize        uint32
			Style         uint32
			LpfnWndProc   uintptr
			CbClsExtra    int32
			CbWndExtra    int32
			HInstance     uintptr
			HIcon         uintptr
			HCursor       uintptr
			HbrBackground uintptr
			LpszMenuName  *uint16
			LpszClassName *uint16
			HIconSm       uintptr
		}

		wndProcCallback = windows.NewCallback(wndProc)
		wc := WNDCLASSEX{
			LpfnWndProc:   wndProcCallback,
			HInstance:     hInstance,
			LpszClassName: hostClass,
		}
		wc.CbSize = uint32(unsafe.Sizeof(wc))
		user32.NewProc("RegisterClassExW").Call(uintptr(unsafe.Pointer(&wc)))
	})
}

// ── WebView2 lifecycle ────────────────────────────────────────────────────

func openOnMonitor(m *MonitorInfo, url string) {
	// Create or reuse the host window
	if state.hwnd == 0 {
		hInstance, _, _ := kernel32.NewProc("GetModuleHandleW").Call(0)
		registerClass(hInstance)

		hwnd, _, _ := user32.NewProc("CreateWindowExW").Call(
			0, // dwExStyle
			uintptr(unsafe.Pointer(hostClass)),
			uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("Customer Display"))),
			0x80000000| // WS_POPUP
				0x00800000| // WS_BORDER (removed for borderless)
				0,
			uintptr(m.X), uintptr(m.Y),
			uintptr(m.Width), uintptr(m.Height),
			0, 0, hInstance, 0,
		)
		if hwnd == 0 {
			return
		}
		state.hwnd = hwnd

		// Create WebView2 inside the host window
		chromium := edge.NewChromium()
		chromium.SetPermission(edge.CoreWebView2PermissionKindClipboardRead, edge.CoreWebView2PermissionStateAllow)
		if ok := chromium.Embed(state.hwnd); !ok {
			return
		}
		chromium.Resize()
		state.wv = chromium
	} else {
		// Reposition window on the new monitor
		procSetWindowPos.Call(
			state.hwnd, 0,
			uintptr(m.X), uintptr(m.Y),
			uintptr(m.Width), uintptr(m.Height),
			SWP_SHOWWINDOW|SWP_NOACTIVATE,
		)
		state.wv.Resize()
	}

	state.currentURL = url
	state.wv.Navigate(url)
	procShowWindow.Call(state.hwnd, SW_SHOW)
}

// ── Platform functions ────────────────────────────────────────────────────

func platformInit() {
	// Enable per-monitor DPI awareness for correct multi-monitor scaling.
	procSetProcessDPIAware.Call()
	// Windows does not have a simple equivalent to GDK monitor-added signals;
	// the Wails runtime handles WM_DISPLAYCHANGE at the app level. We leave
	// this as a no-op placeholder — monitor change callbacks are not fired on
	// Windows in the current architecture.
}

func platformGetMonitors() []MonitorInfo {
	return enumerateMonitors()
}

func platformOpen(monitorID, url string) {
	state.mu.Lock()
	defer state.mu.Unlock()

	var m *MonitorInfo
	if monitorID != "" {
		m = findMonitor(monitorID)
	}
	if m == nil {
		// Fall back to primary monitor
		all := enumerateMonitors()
		for i := range all {
			if all[i].IsPrimary {
				m = &all[i]
				break
			}
		}
		if m == nil && len(all) > 0 {
			m = &all[0]
		}
	}
	if m == nil {
		return
	}
	openOnMonitor(m, url)
}

func platformClose() {
	state.mu.Lock()
	defer state.mu.Unlock()

	if state.hwnd != 0 {
		user32.NewProc("DestroyWindow").Call(state.hwnd)
		state.hwnd = 0
		state.wv = nil
		state.currentURL = ""
	}
}

func platformReload() {
	state.mu.Lock()
	defer state.mu.Unlock()

	if state.wv != nil && state.currentURL != "" {
		state.wv.Navigate(state.currentURL)
	}
}

func platformNavigate(url string) {
	state.mu.Lock()
	defer state.mu.Unlock()

	if state.wv != nil {
		state.currentURL = url
		state.wv.Navigate(url)
	}
}

// identifyHTML returns an HTML page showing the monitor number n.
func identifyHTML(n int) string {
	return fmt.Sprintf(`<!DOCTYPE html><html><head><style>
body{background:rgba(15,23,42,.95);color:#fff;
font-family:system-ui,-apple-system,sans-serif;
display:flex;justify-content:center;align-items:center;
height:100vh;margin:0;overflow:hidden;}
.n{font-size:240px;font-weight:800;
background:linear-gradient(135deg,#a78bfa,#8b5cf6);
-webkit-background-clip:text;-webkit-text-fill-color:transparent;
filter:drop-shadow(0 10px 20px rgba(139,92,246,.3));}
</style></head><body><div class="n">%d</div></body></html>`, n)
}

// testHTML is the static test page content.
const testHTML = `<!DOCTYPE html><html><head><style>
body{background:#0f172a;color:#fff;
font-family:system-ui,-apple-system,sans-serif;
display:flex;flex-direction:column;justify-content:center;align-items:center;
height:100vh;margin:0;overflow:hidden;text-align:center;}
h1{font-size:48px;font-weight:800;letter-spacing:.1em;margin-bottom:24px;
background:linear-gradient(135deg,#a78bfa,#8b5cf6);
-webkit-background-clip:text;-webkit-text-fill-color:transparent;}
p{font-size:20px;color:#94a3b8;max-width:600px;line-height:1.6;}
.line{width:80px;height:4px;background:#8b5cf6;margin:32px auto;border-radius:2px;}
</style></head><body>
<h1>CUSTOMER DISPLAY</h1>
<div class="line"></div>
<p>If you can see this screen,<br>the monitor selection is correct.</p>
<p style="font-size:14px;color:#64748b;margin-top:40px;">This window will close automatically.</p>
</body></html>`

// showHTMLWindow creates a transient, borderless, always-on-top window with a
// WebView2 that loads the provided HTML string, then closes after duration.
func showHTMLWindow(m MonitorInfo, html string, duration time.Duration) {
	hInstance, _, _ := kernel32.NewProc("GetModuleHandleW").Call(0)
	registerClass(hInstance)

	hwnd, _, _ := user32.NewProc("CreateWindowExW").Call(
		0x00000008, // WS_EX_TOPMOST
		uintptr(unsafe.Pointer(hostClass)),
		uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("CD Overlay"))),
		0x80000000, // WS_POPUP
		uintptr(m.X), uintptr(m.Y),
		uintptr(m.Width), uintptr(m.Height),
		0, 0, hInstance, 0,
	)
	if hwnd == 0 {
		return
	}

	chromium := edge.NewChromium()
	if ok := chromium.Embed(hwnd); !ok {
		user32.NewProc("DestroyWindow").Call(hwnd)
		return
	}
	chromium.Resize()

	// Encode html as a data: URL for NavigateToString
	b, _ := json.Marshal(html)
	_ = b
	chromium.NavigateToString(html)
	procShowWindow.Call(hwnd, SW_SHOW)

	time.AfterFunc(duration, func() {
		user32.NewProc("DestroyWindow").Call(hwnd)
	})
}

func platformIdentify() {
	monitors := enumerateMonitors()
	for i, m := range monitors {
		m := m
		n := i + 1
		go showHTMLWindow(m, identifyHTML(n), 3*time.Second)
	}
}

func platformTest(monitorID string) {
	m := findMonitor(monitorID)
	if m == nil {
		all := enumerateMonitors()
		if len(all) > 0 {
			m = &all[0]
		}
	}
	if m == nil {
		return
	}
	go showHTMLWindow(*m, testHTML, 3*time.Second)
}
