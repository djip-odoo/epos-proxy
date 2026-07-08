module epos-proxy

go 1.25.0

require (
	github.com/emersion/go-autostart v0.0.0-20250403115856-34830d6457d2
	github.com/gofiber/fiber/v3 v3.1.0
	github.com/google/gousb v1.1.3
	github.com/sirupsen/logrus v1.9.4
	github.com/wailsapp/wails/v3 v3.0.0-alpha2.116
	github.com/yusufpapurcu/wmi v1.2.4
	golang.org/x/sys v0.43.0
	gopkg.in/natefinch/lumberjack.v2 v2.2.1
)

require (
	github.com/adrg/xdg v0.5.3 // indirect
	github.com/andybalholm/brotli v1.2.0 // indirect
	github.com/coder/websocket v1.8.14 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/godbus/dbus/v5 v5.2.2 // indirect
	github.com/gofiber/schema v1.7.0 // indirect
	github.com/gofiber/utils/v2 v2.0.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/jchv/go-winloader v0.0.0-20250406163304-c1995be93bd1 // indirect
	github.com/klauspost/compress v1.18.4 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/philhofer/fwd v1.2.0 // indirect
	github.com/tinylib/msgp v1.6.3 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasthttp v1.69.0 // indirect
	github.com/wailsapp/wails/webview2 v1.0.27 // indirect
	golang.org/x/crypto v0.50.0 // indirect
	golang.org/x/net v0.53.0 // indirect
	golang.org/x/text v0.37.0 // indirect
)

replace github.com/wailsapp/wails/v3 => ./vendor-patches/wails-v3
