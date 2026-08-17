package server

import (
	"encoding/xml"
	"errors"
	"fmt"
	"net"
	"sync/atomic"

	"epos-proxy/internal/config"
	"epos-proxy/internal/escpos"
	"epos-proxy/internal/logger"
	"epos-proxy/internal/printer"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
)

type EPOSResponse struct {
	XMLName xml.Name `xml:"response"`
	Success bool     `xml:"success,attr"`
	Code    string   `xml:"code,attr"`
	Status  string   `xml:"status,attr"`
}

type Server struct {
	app     *fiber.App
	ln      net.Listener
	Port    int
	running atomic.Bool
}

func New(cfg *config.Manager, mgr *printer.Manager) (*Server, error) {
	if cfg == nil {
		return nil, errors.New("unable to start server: config manager is required")
	}

	port, err := cfg.ResolvePort()
	if err != nil {
		return nil, fmt.Errorf("unable to start server: %w", err)
	}

	ln, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		return nil, fmt.Errorf("unable to start server: %w", err)
	}
	app := fiber.New(fiber.Config{
		AppName: "ePOS proxy",
	})
	app.Use(cors.New(cors.Config{
		AllowOrigins:        []string{"*"},
		AllowPrivateNetwork: true,
	}))

	app.Post("/p/:printerId/cgi-bin/epos/service.cgi", func(ctx fiber.Ctx) error {
		printerId := ctx.Params("printerId")
		logger.Debugf("Print request received for printer: %s", printerId)
		return printData(mgr, ctx, printerId)
	})

	app.Post("/cgi-bin/epos/service.cgi", func(ctx fiber.Ctx) error {
		logger.Debugf("Print request received (auto printer selection)")
		return printData(mgr, ctx, "")
	})

	app.Post("/p/:printerId/pstprnt", func(ctx fiber.Ctx) error {
		printerId := ctx.Params("printerId")
		logger.Debugf("Label print request received for printer: %s", printerId)
		return printLabel(mgr, ctx, printerId)
	})

	server := &Server{app: app, ln: ln, Port: port}
	server.running.Store(true)
	go func() {
		logger.Infof("HTTP server listening on 0.0.0.0:%d", port)
		if err := app.Listener(ln); err != nil {
			logger.Error("EPOS Server Error: ", err)
		}
		server.running.Store(false)
		logger.Warn("HTTP server stopped")
	}()

	return server, nil
}

func printData(mgr *printer.Manager, ctx fiber.Ctx, printerID string) error {
	logger.Debugf("Processing print job for printer: %s", printerID)
	jobData, err := escpos.ParseXML(ctx.Body())
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

func printLabel(mgr *printer.Manager, ctx fiber.Ctx, printerID string) error {
	jobData := ctx.Body()

	if len(jobData) == 0 {
		logger.Warn("Empty label data received")
		return ctx.SendStatus(fiber.StatusBadRequest)
	}

	logger.Debugf("Processing label print job for printer: %s", printerID)

	reply, err := mgr.WriteAsync(printerID, jobData)
	if err == nil {
		logger.Debug("Label print job queued")
		result := <-reply
		if !result.OK {
			err = result.Err
		}
	}

	if err != nil {
		if errors.Is(err, printer.ErrQueueFull) {
			logger.Warnf("Printer queue full, Printer ID: %s", printerID)
			return ctx.SendStatus(fiber.StatusTooManyRequests)
		}

		logger.Errorf("Print error: %v, Printer ID: %s", err, printerID)
		return ctx.SendStatus(fiber.StatusInternalServerError)
	}

	logger.Debugf("Print job completed successfully for printer: %s", printerID)
	return ctx.SendStatus(fiber.StatusOK)
}

func (s *Server) Stop() error {
	logger.Infof("Stopping HTTP server")
	s.running.Store(false)
	var closeErr error
	if s.ln != nil {
		closeErr = s.ln.Close()
	}
	err := s.app.Shutdown()
	if err != nil {
		return err
	}
	return closeErr
}

func (s *Server) Running() bool {
	return s.running.Load()
}

func (s *Server) GetPrinterIp(id string) string {
	ip := fmt.Sprintf("127.0.0.1:%d/p/%s", s.Port, id)
	logger.Debugf("Generated printer endpoint: %s", ip)
	return ip
}
