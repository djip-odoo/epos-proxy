package obox

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"epos-proxy/internal/escpos"
	"epos-proxy/internal/logger"
	"epos-proxy/internal/printer"
)

type QueueAction struct {
	UUID    string                 `json:"uuid"`
	Payload map[string]interface{} `json:"payload"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Name    string `json:"name"`
		Message string `json:"message"`
	} `json:"data"`
}

func (e *rpcError) Error() string {
	if e.Data.Name != "" {
		return fmt.Sprintf("server RPC error %d (%s): %s", e.Code, e.Data.Name, e.Message)
	}
	return fmt.Sprintf("server RPC error %d: %s", e.Code, e.Message)
}

func isDeviceNotFound(err error) bool {
	var rpcErr *rpcError
	if !errors.As(err, &rpcErr) {
		return false
	}
	return rpcErr.Code == http.StatusNotFound
}

func (m *Module) buildDeviceList() []map[string]string {
	discovered := printer.DiscoverAllPrinters(m.cfg)
	list := make([]map[string]string, 0, len(discovered.Available))
	for _, p := range discovered.Available {
		list = append(list, map[string]string{
			"name":       p.Name,
			"identifier": p.Identifier,
			"type":       string(p.Type),
		})
	}
	return list
}

func (m *Module) deviceBrain() {
	logger.Infof("[obox brain] Background polling worker started")
	for {
		time.Sleep(5 * time.Second)

		dbURL, token := m.GetCredentials()
		if dbURL == "" || token == "" {
			m.setLiveStatus("disconnected")
			continue
		}

		actions, err := m.fetchNextActions(dbURL, token)
		if err != nil {
			if isDeviceNotFound(err) {
				logger.Warnf("[obox brain] Device not found on server, disconnecting: %v", err)
				m.Disconnect()
				continue
			}
			logger.Infof("[obox brain] fetchNextActions: %v", err)
			last := m.lastContactTime.Load()
			if last == 0 || time.Since(time.UnixMilli(last)) > 10*time.Second {
				m.setLiveStatus("disconnected")
			} else {
				m.setLiveStatus("connecting")
			}
			continue
		}

		m.setLiveStatus("connected")
		m.lastContactTime.Store(time.Now().UnixMilli())

		for _, action := range actions {
			go m.ExecuteAction(action)
		}
	}
}

func (m *Module) fetchNextActions(dbURL, token string) ([]QueueAction, error) {
	type rpcPayload struct {
		JSONRPC string      `json:"jsonrpc"`
		Method  string      `json:"method"`
		ID      int         `json:"id"`
		Params  interface{} `json:"params"`
	}
	body, _ := json.Marshal(rpcPayload{
		JSONRPC: "2.0",
		Method:  "call",
		ID:      1,
		Params: map[string]string{
			"serial_number": m.appID,
			"token":         token,
		},
	})

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post(dbURL+"/obox/get_next_actions", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected HTTP status %d from /obox/get_next_actions", resp.StatusCode)
	}

	var rpcResp struct {
		Result []QueueAction `json:"result"`
		Error  *rpcError     `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, err
	}
	if rpcResp.Error != nil {
		return nil, rpcResp.Error
	}
	return rpcResp.Result, nil
}

func (m *Module) ExecuteAction(action QueueAction) {
	rawURL, _ := action.Payload["url"].(string)
	method, _ := action.Payload["method"].(string)
	payload := action.Payload["payload"]

	actionPath := rawURL
	if parsed, err := url.Parse(rawURL); err == nil && parsed.Path != "" {
		actionPath = parsed.Path
	}

	logger.Infof("[obox brain] Executing queue action uuid=%s path=%s method=%s", action.UUID, actionPath, method)

	var result interface{}

	switch {
	case actionPath == "/odoo/health":
		logger.Infof("[obox brain] Action health ping: returning success")
		result = map[string]string{"status": "ok"}
		m.reportActionResult(action.UUID, result)
		go m.callOdooPing()
		return

	case actionPath == "/odoo/restart":
		logger.Infof("[obox brain] Action restart requested: not supported on desktop ePOS proxy")
		result = map[string]string{
			"status":  "not_supported",
			"message": "Restart is not supported on ePOS proxy",
		}
		m.reportActionResult(action.UUID, result)
		return

	case actionPath == "/odoo/disconnect":
		logger.Infof("[obox brain] Action disconnect: returning success")
		m.Disconnect()
		result = map[string]string{"status": "disconnected"}
		m.reportActionResult(action.UUID, result)
		return

	case actionPath == "/odoo/discover_devices":
		logger.Infof("[obox brain] Action discover_devices: fetching device list")
		devices := m.buildDeviceList()
		devicesJSON, err := json.Marshal(devices)
		if err == nil {
			result = string(devicesJSON)
		} else {
			result = "[]"
		}
		m.reportActionResult(action.UUID, result)
		return

	case strings.Contains(actionPath, "/cgi-bin/epos/service.cgi"):
		logger.Infof("[obox brain] Action POS ePOS print: executing directly")
		result = m.directEPOSPrint(actionPath, payload)
		m.reportActionResult(action.UUID, result)
		return

	case strings.HasPrefix(actionPath, "/sos/v1/"):
		logger.Infof("[obox brain] Action remote debug: not supported on desktop ePOS proxy")
		result = map[string]string{"error": "remote debug not supported on ePOS proxy"}
		m.reportActionResult(action.UUID, result)
		return

	case actionPath == "/usb/v1/printer/print": // TODO: in office printer
		logger.Infof("[obox brain] Action printer print: executing directly")
		result = map[string]string{"error": "printer print not supported on ePOS proxy"}
		m.reportActionResult(action.UUID, result)
		return

	default:
		logger.Warnf("[obox brain] Action %s: unsupported on desktop ePOS proxy", actionPath)
		result = map[string]string{"error": "action not supported on ePOS proxy"}
		m.reportActionResult(action.UUID, result)
		return
	}
}

func (m *Module) directEPOSPrint(actionPath string, payload interface{}) interface{} {
	printerID := extractPrinterID(actionPath)

	var xmlData []byte
	switch v := payload.(type) {
	case string:
		xmlData = []byte(v)
	case []byte:
		xmlData = v
	case map[string]interface{}:
		if s, ok := v["receipt"].(string); ok {
			xmlData = []byte(s)
		}
	}

	if len(xmlData) == 0 {
		return map[string]string{"error": "empty print payload"}
	}

	jobData, err := escpos.ParseXML(xmlData)
	if err != nil {
		logger.Errorf("[obox brain] XML parsing error: %v", err)
		return map[string]string{"error": err.Error()}
	}

	reply, err := m.mgr.WriteAsync(printerID, jobData)
	if err == nil {
		result := <-reply
		if !result.OK {
			err = result.Err
		}
	}
	if err != nil {
		logger.Errorf("[obox brain] Print error: %v, Printer ID: %s", err, printerID)
		return map[string]string{"error": err.Error()}
	}

	return map[string]string{"status": "ok"}
}

func extractPrinterID(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i, part := range parts {
		if (part == "printer" || part == "p") && i+1 < len(parts) && parts[i+1] != "list" && parts[i+1] != "print" && parts[i+1] != "open-cashbox" && parts[i+1] != "cgi-bin" && parts[i+1] != "pstprnt" {
			return parts[i+1]
		}
	}
	return ""
}

func (m *Module) reportActionResult(uuid string, result interface{}) {
	dbURL, token := m.GetCredentials()
	if dbURL == "" || token == "" {
		return
	}
	type rpcPayload struct {
		JSONRPC string      `json:"jsonrpc"`
		Method  string      `json:"method"`
		ID      int         `json:"id"`
		Params  interface{} `json:"params"`
	}
	body, _ := json.Marshal(rpcPayload{
		JSONRPC: "2.0",
		Method:  "call",
		ID:      1,
		Params: map[string]interface{}{
			"serial_number": m.appID,
			"token":         token,
			"action_uuid":   uuid,
			"result":        result,
		},
	})

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post(dbURL+"/obox/action_result", "application/json", bytes.NewReader(body))
	if err != nil {
		logger.Errorf("[obox brain] action_result error for uuid %s: %v", uuid, err)
		return
	}
	defer resp.Body.Close()
	logger.Infof("[obox brain] action_result response status: %d for uuid %s", resp.StatusCode, uuid)
}

func (m *Module) callOdooPing() {
	dbURL, token := m.GetCredentials()
	if dbURL == "" || token == "" {
		return
	}
	type rpcPayload struct {
		JSONRPC string      `json:"jsonrpc"`
		Method  string      `json:"method"`
		ID      int         `json:"id"`
		Params  interface{} `json:"params"`
	}

	body, _ := json.Marshal(rpcPayload{
		JSONRPC: "2.0",
		Method:  "call",
		ID:      1,
		Params: map[string]string{
			"serial_number": m.appID,
			"token":         token,
		},
	})

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post(dbURL+"/obox/ping", "application/json", bytes.NewReader(body))
	if err != nil {
		logger.Errorf("[obox brain] /obox/ping error: %v", err)
		return
	}
	defer resp.Body.Close()
	logger.Infof("[obox brain] /obox/ping response status: %d", resp.StatusCode)
}

func (m *Module) callOdooOboxConnect(dbURL, token, dbUUID string) {
	endpoint := dbURL + "/obox/connect"
	client := &http.Client{Timeout: 5 * time.Second}

	type rpcPayload struct {
		JSONRPC string      `json:"jsonrpc"`
		Method  string      `json:"method"`
		ID      int         `json:"id"`
		Params  interface{} `json:"params"`
	}

	for attempt := 1; attempt <= 10; attempt++ {
		time.Sleep(time.Duration(attempt*300) * time.Millisecond)

		payload := rpcPayload{
			JSONRPC: "2.0",
			Method:  "call",
			ID:      1,
			Params: map[string]interface{}{
				"serial_number": m.appID,
				"token":         token,
				"local_ip":      m.localAddrFn(),
				"services":      []string{"usb", "printer"},
			},
		}

		body, err := json.Marshal(payload)
		if err != nil {
			logger.Errorf("[obox] callOdooOboxConnect marshal error: %v", err)
			return
		}

		resp, err := client.Post(endpoint, "application/json", bytes.NewReader(body))
		if err != nil {
			logger.Warnf("[obox] /obox/connect attempt %d connection error: %v", attempt, err)
			continue
		}

		var rpcResp struct {
			Result *json.RawMessage `json:"result"`
			Error  *json.RawMessage `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&rpcResp)
		resp.Body.Close()

		if resp.StatusCode == 200 && rpcResp.Error == nil {
			logger.Infof("[obox] Odoo /obox/connect SUCCESS on attempt %d (paired as %s)", attempt, m.appID)
			m.SetCredentials(dbURL, token, dbUUID)
			if m.cfg != nil {
				_ = m.cfg.SetOdooCredentials(dbURL, token, dbUUID)
			}
			m.setLiveStatus("connected")
			return
		}

		if rpcResp.Error != nil {
			logger.Warnf("[obox] /obox/connect (serial=%s) error from Odoo: %s", m.appID, string(*rpcResp.Error))
		} else {
			logger.Warnf("[obox] /obox/connect (serial=%s) HTTP %d", m.appID, resp.StatusCode)
		}
	}

	logger.Errorf("[obox] Failed to complete /obox/connect after 10 attempts")
}
