package mock

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPingReturnsExpectedJSONShape(t *testing.T) {
	handler := newTestHandler()

	resp := performRequest(t, handler, http.MethodGet, "/api/v3/ping?omsId=mock-oms-local", nil, nil)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
	}

	var body struct {
		OMSId      string `json:"omsId"`
		APIVersion string `json:"apiVersion"`
		OMSVersion string `json:"omsVersion"`
	}
	decodeJSON(t, resp.Body, &body)

	if body.OMSId != "mock-oms-local" {
		t.Fatalf("omsId = %q, want %q", body.OMSId, "mock-oms-local")
	}
	if body.APIVersion == "" {
		t.Fatal("apiVersion is empty")
	}
	if body.OMSVersion == "" {
		t.Fatal("omsVersion is empty")
	}
}

func TestCreateOrderReturnsDeterministicOrderID(t *testing.T) {
	t.Setenv("SUZ_MOCK_DYNAMIC_IDS", "false")
	t.Setenv("SUZ_MOCK_ORDER_ID", "11111111-1111-4111-8111-111111111111")

	handler := newTestHandler()
	body := []byte(`{"productGroup":"mockpharma","products":[{"gtin":"00000000000000","quantity":20,"templateId":50}]}`)

	resp := performRequest(t, handler, http.MethodPost, "/api/v3/order?omsId=mock-oms-local", body, map[string]string{
		"Content-Type": "application/json",
	})

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
	}

	var parsed struct {
		OMSId   string `json:"omsId"`
		OrderID string `json:"orderId"`
	}
	decodeJSON(t, resp.Body, &parsed)

	if parsed.OMSId != "mock-oms-local" {
		t.Fatalf("omsId = %q, want %q", parsed.OMSId, "mock-oms-local")
	}
	if parsed.OrderID != "11111111-1111-4111-8111-111111111111" {
		t.Fatalf("orderId = %q", parsed.OrderID)
	}
}

func TestForcedErrorReturnsAPIError(t *testing.T) {
	handler := newTestHandler()

	resp := performRequest(t, handler, http.MethodGet, "/api/v3/ping?omsId=mock-oms-local&__error=500", nil, nil)

	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusInternalServerError)
	}

	var apiErr APIError
	decodeJSON(t, resp.Body, &apiErr)

	if apiErr.Success {
		t.Fatal("success = true, want false")
	}
	if len(apiErr.GlobalErrors) != 1 {
		t.Fatalf("globalErrors len = %d, want 1", len(apiErr.GlobalErrors))
	}
	if apiErr.GlobalErrors[0].ErrorCode != http.StatusInternalServerError {
		t.Fatalf("errorCode = %d, want %d", apiErr.GlobalErrors[0].ErrorCode, http.StatusInternalServerError)
	}
}

func TestRequireTokenRejectsMissingToken(t *testing.T) {
	t.Setenv("SUZ_MOCK_REQUIRE_TOKEN", "true")

	handler := newTestHandler()

	resp := performRequest(t, handler, http.MethodGet, "/api/v3/ping?omsId=mock-oms-local", nil, nil)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusUnauthorized)
	}
}

func newTestHandler() http.Handler {
	srv := NewServer(Config{
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		AccessLog: false,
	})
	return srv.Handler
}

func performRequest(t *testing.T, handler http.Handler, method, target string, body []byte, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	return resp
}

func decodeJSON(t *testing.T, body *bytes.Buffer, target any) {
	t.Helper()

	if err := json.NewDecoder(body).Decode(target); err != nil {
		t.Fatalf("decode JSON: %v\nbody: %s", err, body.String())
	}
}
