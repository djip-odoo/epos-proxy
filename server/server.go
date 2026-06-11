package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"epos-proxy/config"
	"epos-proxy/escpos"
	"epos-proxy/logger"
	"epos-proxy/printer"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/static"
)

type Printer struct {
	Name   string `json:"name"`
	Ip     string `json:"ip"`
	Id     string `json:"id"`
	IsLAN  bool   `json:"isLAN"`
	LANIp  string `json:"lanIp,omitempty"`
	Online bool   `json:"online"`
}

type UnavailablePrinter struct {
	Name     string `json:"name"`
	ErrorMsg string `json:"errorMsg"`
	IsLAN    bool   `json:"isLAN"`
	LANIp    string `json:"lanIp,omitempty"`
}

type Status struct {
	ServerRunning       bool                 `json:"serverRunning"`
	DefaultIp           string               `json:"defaultIp"`
	ErrorMsg            string               `json:"errorMsg"`
	Printers            []Printer            `json:"printers"`
	UnavailablePrinters []UnavailablePrinter `json:"unavailablePrinters"`
	Os                  string               `json:"os"`
}

type LANSettings struct {
	Enabled bool   `json:"enabled"`
	IP      string `json:"ip"`
	Port    int    `json:"port"`
}

type Service interface {
	Status() (Status, error)
	AddLANPrinter(ip string) error
	RemoveLANPrinter(ip string) error
	CheckLANPrinterStatus(ip string) bool
	GetLANSettings() LANSettings
	EnableLANAccess() error
	DisableLANAccess() error
	IsAutostartEnabled() bool
	EnableAutostart() error
	DisableAutostart() error
	GetPrinterSetting(id string) config.PrinterSettingConfig
	SetPrinterSetting(id string, width int, bottomPadding int, protocol string) error
	OpenCashDrawer(printerId string) error
	ExportLogsToWriter(w io.Writer) error
	ExportLogsToPath(savePath string) error
	GetPrinterIp(id string) string
	GetLocalIPAddress() string
	GetPrinterManager() *printer.Manager
	GetLANPin() string
	SetLANPin(pin string) error
	// Customer Display WebView
	GetCustomerDisplayURLs() []config.CustomerDisplayURL
	GetActiveCustomerDisplayURL() *config.CustomerDisplayURL
	AddCustomerDisplayURL(name, rawURL, description string) (config.CustomerDisplayURL, error)
	SetActiveCustomerDisplayURL(id string) error
	DeleteCustomerDisplayURL(id string) error
	ValidateAdminPin(pin string) bool
	IsCustomerDisplayOpen() bool
	SetCustomerDisplayOpen(open bool) error
}

type EPOSResponse struct {
	XMLName xml.Name `xml:"response"`
	Success bool     `xml:"success,attr"`
	Code    string   `xml:"code,attr"`
	Status  string   `xml:"status,attr"`
}

type Server struct {
	app     *fiber.App
	Port    int
	running atomic.Bool
	service Service
}

func New(port int, host string, svc Service, assets fs.FS) *Server {
	logger.Infof("Server initializing on port %d", port)
	app := fiber.New(fiber.Config{
		AppName: "ePOS proxy",
	})
	app.Use(cors.New(cors.Config{
		AllowOrigins:        []string{"*"},
		AllowPrivateNetwork: true,
	}))

	// Custom LAN Access PIN authentication middleware
	app.Use(func(c fiber.Ctx) error {
		path := c.Path()
		// Only protect API routes, except for /api/ping and /api/verify-pin
		if !strings.HasPrefix(path, "/api/") || path == "/api/ping" || path == "/api/verify-pin" {
			return c.Next()
		}

		// Check if request is local (127.0.0.1 or ::1)
		clientIP := c.IP()
		if clientIP == "127.0.0.1" || clientIP == "::1" {
			return c.Next()
		}

		// Remote request: check if a PIN is configured
		pin := svc.GetLANPin()
		if pin == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"code":    "PIN_NOT_SET",
				"message": "LAN access PIN has not been set by the administrator yet.",
			})
		}

		// Check the token in X-LAN-Token header
		token := c.Get("X-LAN-Token")
		if token == "" || !isValidToken(token) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"code":    "PIN_REQUIRED",
				"message": "PIN verification required",
			})
		}

		return c.Next()
	})

	app.Post("/p/:printerId/cgi-bin/epos/service.cgi", func(ctx fiber.Ctx) error {
		printerId := ctx.Params("printerId")
		logger.Debugf("Print request received for printer: %s", printerId)
		return printData(svc.GetPrinterManager(), ctx, printerId)
	})

	app.Post("/cgi-bin/epos/service.cgi", func(ctx fiber.Ctx) error {
		logger.Debugf("Print request received (auto printer selection)")
		return printData(svc.GetPrinterManager(), ctx, "")
	})

	// REST API Routes
	app.Get("/api/ping", func(ctx fiber.Ctx) error {
		return ctx.JSON(fiber.Map{"status": "ok"})
	})

	app.Post("/api/verify-pin", func(ctx fiber.Ctx) error {
		var req struct {
			Pin string `json:"pin"`
		}
		if err := ctx.Bind().JSON(&req); err != nil {
			return apiError(ctx, 400, "BAD_REQUEST", err)
		}

		configuredPin := svc.GetLANPin()
		if configuredPin == "" {
			return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"code":    "PIN_NOT_SET",
				"message": "LAN access PIN has not been set by the administrator yet.",
			})
		}

		if req.Pin != configuredPin {
			return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"code":    "INVALID_PIN",
				"message": "Invalid PIN",
			})
		}

		token, err := generateRandomToken()
		if err != nil {
			return apiError(ctx, 500, "TOKEN_GENERATION_FAILED", err)
		}

		addToken(token)

		return ctx.JSON(fiber.Map{
			"token": token,
		})
	})

	app.Get("/api/status", func(ctx fiber.Ctx) error {
		status, err := svc.Status()
		if err != nil {
			return apiError(ctx, 500, "STATUS_ERROR", err)
		}
		return ctx.JSON(status)
	})

	app.Post("/api/printers/lan", func(ctx fiber.Ctx) error {
		var req struct {
			IP string `json:"ip"`
		}
		if err := ctx.Bind().JSON(&req); err != nil {
			return apiError(ctx, 400, "BAD_REQUEST", err)
		}
		if err := svc.AddLANPrinter(req.IP); err != nil {
			return apiError(ctx, 400, "ADD_PRINTER_FAILED", err)
		}
		return ctx.JSON(fiber.Map{"status": "ok"})
	})

	app.Delete("/api/printers/lan", func(ctx fiber.Ctx) error {
		ip := ctx.Query("ip")
		if ip == "" {
			return apiError(ctx, 400, "BAD_REQUEST", fmt.Errorf("ip parameter is required"))
		}
		if err := svc.RemoveLANPrinter(ip); err != nil {
			return apiError(ctx, 500, "REMOVE_PRINTER_FAILED", err)
		}
		return ctx.JSON(fiber.Map{"status": "ok"})
	})

	app.Get("/api/printers/lan/status", func(ctx fiber.Ctx) error {
		ip := ctx.Query("ip")
		if ip == "" {
			return apiError(ctx, 400, "BAD_REQUEST", fmt.Errorf("ip parameter is required"))
		}
		online := svc.CheckLANPrinterStatus(ip)
		return ctx.JSON(fiber.Map{"online": online})
	})

	app.Get("/api/settings/lan", func(ctx fiber.Ctx) error {
		return ctx.JSON(svc.GetLANSettings())
	})

	app.Post("/api/settings/lan/enable", func(ctx fiber.Ctx) error {
		if err := svc.EnableLANAccess(); err != nil {
			return apiError(ctx, 500, "FIREWALL_ERROR", err)
		}
		return ctx.JSON(fiber.Map{"status": "ok"})
	})

	app.Post("/api/settings/lan/disable", func(ctx fiber.Ctx) error {
		if err := svc.DisableLANAccess(); err != nil {
			return apiError(ctx, 500, "FIREWALL_ERROR", err)
		}
		return ctx.JSON(fiber.Map{"status": "ok"})
	})

	app.Get("/api/settings/autostart", func(ctx fiber.Ctx) error {
		return ctx.JSON(fiber.Map{"enabled": svc.IsAutostartEnabled()})
	})

	app.Post("/api/settings/autostart/enable", func(ctx fiber.Ctx) error {
		if err := svc.EnableAutostart(); err != nil {
			return apiError(ctx, 500, "AUTOSTART_ERROR", err)
		}
		return ctx.JSON(fiber.Map{"status": "ok"})
	})

	app.Post("/api/settings/autostart/disable", func(ctx fiber.Ctx) error {
		if err := svc.DisableAutostart(); err != nil {
			return apiError(ctx, 500, "AUTOSTART_ERROR", err)
		}
		return ctx.JSON(fiber.Map{"status": "ok"})
	})

	app.Get("/api/settings/printer/:id", func(ctx fiber.Ctx) error {
		id := ctx.Params("id")
		return ctx.JSON(svc.GetPrinterSetting(id))
	})

	app.Post("/api/settings/printer", func(ctx fiber.Ctx) error {
		var req struct {
			ID            string `json:"id"`
			Width         int    `json:"width"`
			BottomPadding int    `json:"bottom_padding"`
			Protocol      string `json:"protocol"`
			CashDrawerPin int    `json:"cash_drawer_pin"`
		}
		if err := ctx.Bind().JSON(&req); err != nil {
			return apiError(ctx, 400, "BAD_REQUEST", err)
		}
		if err := svc.SetPrinterSetting(req.ID, req.Width, req.BottomPadding, req.Protocol); err != nil {
			return apiError(ctx, 500, "SAVE_SETTINGS_FAILED", err)
		}
		return ctx.JSON(fiber.Map{"status": "ok"})
	})

	app.Post("/api/print", func(ctx fiber.Ctx) error {
		var req struct {
			PrinterID string `json:"printer_id"`
			XML       string `json:"xml"`
		}
		if err := ctx.Bind().JSON(&req); err != nil {
			return apiError(ctx, 400, "BAD_REQUEST", err)
		}
		psc := config.GetPrinterSetting(req.PrinterID)
		jobData, err := escpos.ParseXML([]byte(req.XML), psc)
		if err != nil {
			return apiError(ctx, 400, "XML_PARSE_ERROR", err)
		}
		reply, err := svc.GetPrinterManager().WriteAsync(req.PrinterID, jobData)
		if err == nil {
			res := <-reply
			if !res.OK {
				err = res.Err
			}
		}
		if err != nil {
			code := "PRINT_FAILED"
			if errors.Is(err, printer.ErrQueueFull) {
				code = "QUEUE_FULL"
			}
			return apiError(ctx, 500, code, err)
		}
		return ctx.JSON(fiber.Map{"status": "ok"})
	})

	app.Post("/api/cashdrawer", func(ctx fiber.Ctx) error {
		var req struct {
			PrinterID string `json:"printer_id"`
		}
		if err := ctx.Bind().JSON(&req); err != nil {
			return apiError(ctx, 400, "BAD_REQUEST", err)
		}
		if err := svc.OpenCashDrawer(req.PrinterID); err != nil {
			return apiError(ctx, 500, "CASHDRAWER_FAILED", err)
		}
		return ctx.JSON(fiber.Map{"status": "ok"})
	})

	app.Get("/api/logs/download", func(ctx fiber.Ctx) error {
		zipName := fmt.Sprintf("epos-proxy-logs-%s.zip", time.Now().Format("2006-01-02"))
		ctx.Response().Header.Set("Content-Type", "application/zip")
		ctx.Response().Header.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", zipName))
		err := svc.ExportLogsToWriter(ctx.Response().BodyWriter())
		if err != nil {
			logger.Errorf("Failed to stream logs: %v", err)
			return err
		}
		return nil
	})

	// ── Customer Display WebView ──────────────────────────────────────────────

	// GET /api/customer-display/urls — list all configured URLs
	app.Get("/api/customer-display/urls", func(ctx fiber.Ctx) error {
		urls := svc.GetCustomerDisplayURLs()
		if urls == nil {
			urls = []config.CustomerDisplayURL{}
		}
		return ctx.JSON(urls)
	})

	// GET /api/customer-display/active — return the active URL or null
	app.Get("/api/customer-display/active", func(ctx fiber.Ctx) error {
		active := svc.GetActiveCustomerDisplayURL()
		if active == nil {
			return ctx.JSON(nil)
		}
		return ctx.JSON(active)
	})

	// POST /api/customer-display/urls — add a new URL (becomes active)
	app.Post("/api/customer-display/urls", func(ctx fiber.Ctx) error {
		var req struct {
			Name        string `json:"name"`
			URL         string `json:"url"`
			Description string `json:"description"`
		}
		if err := ctx.Bind().JSON(&req); err != nil {
			return apiError(ctx, 400, "BAD_REQUEST", err)
		}
		record, err := svc.AddCustomerDisplayURL(req.Name, req.URL, req.Description)
		if err != nil {
			return apiError(ctx, 400, "VALIDATION_ERROR", err)
		}
		// Immediately activate the new URL
		if err := svc.SetActiveCustomerDisplayURL(record.ID); err != nil {
			logger.Warnf("Added customer display URL but failed to set active: %v", err)
		}
		return ctx.Status(fiber.StatusCreated).JSON(record)
	})

	// POST /api/customer-display/urls/:id/activate — set a URL as active
	app.Post("/api/customer-display/urls/:id/activate", func(ctx fiber.Ctx) error {
		id := ctx.Params("id")
		if err := svc.SetActiveCustomerDisplayURL(id); err != nil {
			return apiError(ctx, 400, "NOT_FOUND", err)
		}
		return ctx.JSON(fiber.Map{"status": "ok"})
	})

	// DELETE /api/customer-display/urls/:id — delete a URL
	app.Delete("/api/customer-display/urls/:id", func(ctx fiber.Ctx) error {
		id := ctx.Params("id")
		if err := svc.DeleteCustomerDisplayURL(id); err != nil {
			return apiError(ctx, 400, "NOT_FOUND", err)
		}
		return ctx.JSON(fiber.Map{"status": "ok"})
	})

	// POST /api/customer-display/validate-pin — validate admin PIN
	app.Post("/api/customer-display/validate-pin", func(ctx fiber.Ctx) error {
		var req struct {
			Pin string `json:"pin"`
		}
		if err := ctx.Bind().JSON(&req); err != nil {
			return apiError(ctx, 400, "BAD_REQUEST", err)
		}
		valid := svc.ValidateAdminPin(req.Pin)
		if !valid {
			return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"code":    "INVALID_PIN",
				"message": "Incorrect PIN",
			})
		}
		return ctx.JSON(fiber.Map{"valid": true})
	})

	// GET /api/customer-display/state — get current open/close state of customer display
	app.Get("/api/customer-display/state", func(ctx fiber.Ctx) error {
		return ctx.JSON(fiber.Map{"open": svc.IsCustomerDisplayOpen()})
	})

	// POST /api/customer-display/open — open customer display on the desktop app
	app.Post("/api/customer-display/open", func(ctx fiber.Ctx) error {
		if err := svc.SetCustomerDisplayOpen(true); err != nil {
			return apiError(ctx, 500, "LAUNCH_FAILED", err)
		}
		return ctx.JSON(fiber.Map{"status": "ok"})
	})

	// POST /api/customer-display/close — close customer display on the desktop app
	app.Post("/api/customer-display/close", func(ctx fiber.Ctx) error {
		if err := svc.SetCustomerDisplayOpen(false); err != nil {
			return apiError(ctx, 500, "CLOSE_FAILED", err)
		}
		return ctx.JSON(fiber.Map{"status": "ok"})
	})

	if assets != nil {
		sub, err := fs.Sub(assets, "frontend/dist")
		if err == nil {
			app.Get("/*", static.New("", static.Config{
				FS: sub,
			}))
		} else {
			logger.Errorf("Failed to create sub-filesystem: %v", err)
		}
	}

	server := &Server{app: app, Port: port, service: svc}
	server.running.Store(true)
	go func() {
		bindAddr := fmt.Sprintf("%s:%d", host, port)
		logger.Infof("HTTP server listening on %s", bindAddr)
		err := app.Listen(bindAddr)
		if err != nil {
			logger.Error("EPOS Server Error: ", err)
		}
		server.running.Store(false)
		logger.Warn("HTTP server stopped")
	}()
	return server
}

func apiError(ctx fiber.Ctx, status int, code string, err error) error {
	return ctx.Status(status).JSON(fiber.Map{
		"code":    code,
		"message": err.Error(),
	})
}

func printData(mgr *printer.Manager, ctx fiber.Ctx, printerID string) error {
	logger.Debugf("Processing print job for printer: %s", printerID)
	psc := config.GetPrinterSetting(printerID)
	jobData, err := escpos.ParseXML(ctx.Body(), psc)
	if err != nil {
		logger.Errorf("XML parsing error: %v", err)
		return ctx.XML(EPOSResponse{Success: false, Code: "SchemaError", Status: ""})
	}
	logger.Debug("XML parsed successfully")

	reply, err := mgr.WriteAsync(printerID, jobData)
	if err == nil {
		logger.Debug("Print job queued")
		result := <-reply
		if !result.OK {
			err = result.Err
		}
	}
	if err != nil {
		retCode := ""
		if errors.Is(err, printer.ErrQueueFull) {
			retCode = "TooManyRequests"
			logger.Warn("Printer queue full")
		} else {
			retCode = "EX_BADPORT"
		}
		logger.Errorf("Print error [%s]: %v, Printer ID: %s", retCode, err, printerID)
		return ctx.XML(EPOSResponse{Success: false, Code: retCode, Status: ""})
	}
	logger.Debugf("Print job completed successfully for printer: %s", printerID)
	return ctx.XML(EPOSResponse{Success: true, Code: "", Status: ""})
}

func (s *Server) Stop() error {
	logger.Infof("Stopping HTTP server")
	return s.app.Shutdown()
}

func (s *Server) Running() bool {
	return s.running.Load()
}

var (
	tokens   = make(map[string]time.Time)
	tokensMu sync.RWMutex
)

func isValidToken(token string) bool {
	tokensMu.RLock()
	expiry, exists := tokens[token]
	tokensMu.RUnlock()
	return exists && time.Now().Before(expiry)
}

func addToken(token string) {
	tokensMu.Lock()
	tokens[token] = time.Now().Add(24 * time.Hour)
	tokensMu.Unlock()
}

func generateRandomToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
