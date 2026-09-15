package server

import (
	"errors"

	"epos-proxy/internal/escpos"
	"epos-proxy/internal/logger"
	"epos-proxy/internal/printer"

	"github.com/gofiber/fiber/v3"
)

func registerEPOSRoutes(s *Server) {
	s.app.Post("/p/:printerId/cgi-bin/epos/service.cgi", func(ctx fiber.Ctx) error {
		printerId := ctx.Params("printerId")
		logger.Debugf("Print request received for printer: %s", printerId)
		return printReceipt(s.printer, ctx, printerId)
	})

	s.app.Post("/cgi-bin/epos/service.cgi", func(ctx fiber.Ctx) error {
		logger.Debugf("Print request received (auto printer selection)")
		return printReceipt(s.printer, ctx, "")
	})

	s.app.Post("/p/:printerId/pstprnt", func(ctx fiber.Ctx) error {
		printerId := ctx.Params("printerId")
		logger.Debugf("Label print request received for printer: %s", printerId)
		return printLabel(s.printer, ctx, printerId)
	})
}

func printReceipt(printerMgr *printer.Manager, ctx fiber.Ctx, printerID string) error {
	if printerMgr == nil {
		logger.Errorf("Printer manager is not initialized")
		return ctx.XML(escpos.NewErrorResponse("EX_BADPORT"))
	}

	logger.Debugf("Processing print job for printer: %s", printerID)
	jobData, err := escpos.ParseXML(ctx.Body())
	if err != nil {
		logger.Errorf("XML parsing error: %v", err)
		return ctx.XML(escpos.NewErrorResponse("SchemaError"))
	}
	logger.Debug("XML parsed successfully")

	err = printerMgr.Print(printerID, jobData)
	if err != nil {
		retCode := ""
		if errors.Is(err, printer.ErrQueueFull) {
			retCode = "TooManyRequests"
			logger.Warn("Printer queue full")
		} else {
			retCode = "EX_BADPORT"
		}
		logger.Errorf("Print error [%s]: %v, Printer ID: %s", retCode, err, printerID)
		return ctx.XML(escpos.NewErrorResponse(retCode))
	}

	logger.Debugf("Print job completed successfully for printer: %s", printerID)
	return ctx.XML(escpos.NewSuccessResponse())
}

func printLabel(printerMgr *printer.Manager, ctx fiber.Ctx, printerID string) error {
	jobData := ctx.Body()

	if len(jobData) == 0 {
		logger.Warn("Empty label data received")
		return ctx.SendStatus(fiber.StatusBadRequest)
	}

	logger.Debugf("Processing label print job for printer: %s", printerID)

	err := printerMgr.Print(printerID, jobData)
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
