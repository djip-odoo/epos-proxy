package obox

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"epos-proxy/internal/config"
	"epos-proxy/internal/printer"
	"epos-proxy/internal/testutil"

	"github.com/gofiber/fiber/v3"
)

func createTestModule(t *testing.T) (*Module, *fiber.App) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	cfg, err := config.NewManager()
	testutil.ExpectedNoError(t, err)
	app := fiber.New()

	m := Manager(cfg, printer.NewManager(), func() string { return "127.0.0.1:4545" })
	app.Get("/odoo/", m.HandleDiscovery)
	app.Get("/odoo/connect", m.HandleConnect)
	return m, app
}

func TestObox_CredentialsAndConnection(t *testing.T) {
	m, _ := createTestModule(t)

	testutil.ExpectedFalse(t, m.IsConnected())
	testutil.ExpectedEqual(t, m.GetWebsocketStatus(), "disconnected")

	m.SetCredentials("http://127.0.0.1:8069", "token-xyz", "db-uuid-1")
	testutil.ExpectedTrue(t, m.IsConnected())

	dbURL, tok := m.GetCredentials()
	testutil.ExpectedEqual(t, dbURL, "http://127.0.0.1:8069")
	testutil.ExpectedEqual(t, tok, "token-xyz")
	testutil.ExpectedEqual(t, m.GetDbURL(), "http://127.0.0.1:8069")

	m.ClearCredentials()
	testutil.ExpectedFalse(t, m.IsConnected())
	testutil.ExpectedEqual(t, m.GetDbURL(), "")
}

func TestObox_StatusChangeListener(t *testing.T) {
	m, _ := createTestModule(t)

	called := false
	m.OnStatusChange(func() {
		called = true
	})

	m.setLiveStatus("connected")
	testutil.ExpectedTrue(t, called)
}

func TestObox_Routes(t *testing.T) {
	m, app := createTestModule(t)

	// 1. Initial /odoo/ (not configured)
	req := httptest.NewRequest("GET", "/odoo/", nil)
	resp, err := app.Test(req)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, resp.StatusCode, http.StatusOK)
	resp.Body.Close()

	// 2. /odoo/connect (offline connect)
	req = httptest.NewRequest("GET", "/odoo/connect?db_url=http://127.0.0.1:8069&token=test-tok&db_uuid=test-uuid", nil)
	resp, err = app.Test(req)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, resp.StatusCode, http.StatusOK)
	resp.Body.Close()

	testutil.ExpectedTrue(t, m.IsConnected())

	// 3. /odoo/ configured discovery
	req = httptest.NewRequest("GET", "/odoo/", nil)
	resp, err = app.Test(req)
	testutil.ExpectedNoError(t, err)
	var discResp struct {
		Status string            `json:"status"`
		Data   map[string]string `json:"data"`
	}
	err = json.NewDecoder(resp.Body).Decode(&discResp)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, discResp.Status, "configured")
	testutil.ExpectedEqual(t, discResp.Data["db_url"], "http://127.0.0.1:8069")
	resp.Body.Close()

	// 4. In-memory Disconnect
	m.Disconnect()
	testutil.ExpectedFalse(t, m.IsConnected())
}

func TestObox_ExecuteAction(t *testing.T) {
	actionReported := make(chan string, 5)
	mockOdoo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				ActionUUID string      `json:"action_uuid"`
				Result     interface{} `json:"result"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Params.ActionUUID != "" {
			actionReported <- req.Params.ActionUUID
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"result": "ok"})
	}))
	defer mockOdoo.Close()

	m, _ := createTestModule(t)
	m.SetCredentials(mockOdoo.URL, "tok", "uuid")

	action := QueueAction{
		UUID: "action-1",
		Payload: map[string]interface{}{
			"url":    "/odoo/health",
			"method": "GET",
		},
	}
	m.ExecuteAction(action)

	select {
	case uuid := <-actionReported:
		testutil.ExpectedEqual(t, uuid, "action-1")
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for action report")
	}

	// Test ePOS direct print action
	actionEpos := QueueAction{
		UUID: "action-2",
		Payload: map[string]interface{}{
			"url":     "/usb/v1/printer/printer_1/cgi-bin/epos/service.cgi",
			"method":  "POST",
			"payload": `<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><epos-print xmlns="http://www.epson-pos.com/schemas/2011/03/epos-print"><text>Hello</text></epos-print></s:Body></s:Envelope>`,
		},
	}
	m.ExecuteAction(actionEpos)

	select {
	case uuid := <-actionReported:
		testutil.ExpectedEqual(t, uuid, "action-2")
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for epos action report")
	}

	// Test Document direct print action
	actionDoc := QueueAction{
		UUID: "action-3",
		Payload: map[string]interface{}{
			"url":    "/usb/v1/printer/print",
			"method": "POST",
			"payload": map[string]interface{}{
				"identifier": "printer_1",
				"document":   "bW9jay1kb2M=",
			},
		},
	}
	m.ExecuteAction(actionDoc)

	select {
	case uuid := <-actionReported:
		testutil.ExpectedEqual(t, uuid, "action-3")
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for doc action report")
	}
}

func TestObox_IsDeviceNotFound(t *testing.T) {
	// 1. Non-rpcError
	testutil.ExpectedFalse(t, isDeviceNotFound(errors.New("regular network error")))
	testutil.ExpectedFalse(t, isDeviceNotFound(nil))

	// 2. rpcError with 404 code
	err404 := &rpcError{Code: 404, Message: "404: Not Found"}
	testutil.ExpectedTrue(t, isDeviceNotFound(err404))
	testutil.ExpectedContains(t, err404.Error(), "404")

	// 3. rpcError with werkzeug NotFound exception name
	errWerkzeug := &rpcError{
		Code:    200,
		Message: "Odoo Error",
		Data: struct {
			Name    string `json:"name"`
			Message string `json:"message"`
		}{
			Name:    "werkzeug.exceptions.NotFound",
			Message: "404 Not Found",
		},
	}
	testutil.ExpectedTrue(t, isDeviceNotFound(errWerkzeug))
	testutil.ExpectedContains(t, errWerkzeug.Error(), "werkzeug.exceptions.NotFound")

	// 4. Other RPC error (e.g. 500 Internal Server Error or AccessDenied)
	err500 := &rpcError{Code: 500, Message: "Internal Server Error"}
	testutil.ExpectedFalse(t, isDeviceNotFound(err500))
}
