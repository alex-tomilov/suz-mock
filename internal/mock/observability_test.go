package mock

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"testing"
)

func TestLoadConfigFromEnvDefaults(t *testing.T) {
	t.Setenv("SUZ_MOCK_ADDR", "")
	t.Setenv("SUZ_MOCK_LOG_LEVEL", "")
	t.Setenv("SUZ_MOCK_ACCESS_LOG", "")
	t.Setenv("SUZ_MOCK_JSON_LOGS", "")

	cfg := LoadConfigFromEnv()

	if cfg.Addr != ":8080" {
		t.Fatalf("Addr = %q, want :8080", cfg.Addr)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Fatalf("LogLevel = %v, want info", cfg.LogLevel)
	}
	if !cfg.AccessLog {
		t.Fatal("AccessLog = false, want true")
	}
	if cfg.JSONLogs {
		t.Fatal("JSONLogs = true, want false")
	}
	if cfg.Logger == nil {
		t.Fatal("Logger is nil")
	}
}

func TestLoadConfigFromEnvObservabilityOverrides(t *testing.T) {
	t.Setenv("SUZ_MOCK_ADDR", ":9090")
	t.Setenv("SUZ_MOCK_LOG_LEVEL", "debug")
	t.Setenv("SUZ_MOCK_ACCESS_LOG", "false")
	t.Setenv("SUZ_MOCK_JSON_LOGS", "true")

	cfg := LoadConfigFromEnv()

	if cfg.Addr != ":9090" {
		t.Fatalf("Addr = %q, want :9090", cfg.Addr)
	}
	if cfg.LogLevel != slog.LevelDebug {
		t.Fatalf("LogLevel = %v, want debug", cfg.LogLevel)
	}
	if cfg.AccessLog {
		t.Fatal("AccessLog = true, want false")
	}
	if !cfg.JSONLogs {
		t.Fatal("JSONLogs = false, want true")
	}
}

func TestParseLogLevelFallsBackToInfo(t *testing.T) {
	if level := parseLogLevel("unexpected"); level != slog.LevelInfo {
		t.Fatalf("level = %v, want info", level)
	}
}

func TestJSONLoggerWritesJSON(t *testing.T) {
	var buf bytes.Buffer
	logger := newLogger(&buf, slog.LevelDebug, true)

	logger.Debug("debug event", "key", "value")

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("log is not JSON: %v\n%s", err, buf.String())
	}
	if line["msg"] != "debug event" {
		t.Fatalf("msg = %v, want debug event", line["msg"])
	}
	if line["key"] != "value" {
		t.Fatalf("key = %v, want value", line["key"])
	}
}

func TestAccessLogCanBeDisabled(t *testing.T) {
	var buf bytes.Buffer
	srv := NewServer(Config{
		Logger:    newLogger(&buf, slog.LevelInfo, false),
		AccessLog: false,
	})

	resp := performRequest(t, srv.Handler, http.MethodGet, "/api/v3/ping?omsId=mock-oms-local", nil, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
	}
	if buf.Len() != 0 {
		t.Fatalf("log output = %q, want empty", buf.String())
	}
}

func TestAccessLogCanBeEnabled(t *testing.T) {
	var buf bytes.Buffer
	srv := NewServer(Config{
		Logger:    newLogger(&buf, slog.LevelInfo, false),
		AccessLog: true,
	})

	resp := performRequest(t, srv.Handler, http.MethodGet, "/api/v3/ping?omsId=mock-oms-local", nil, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
	}
	if !bytes.Contains(buf.Bytes(), []byte("msg=request")) {
		t.Fatalf("log output = %q, want request log", buf.String())
	}
}

func TestMockStateAndReset(t *testing.T) {
	t.Setenv("SUZ_MOCK_DYNAMIC_IDS", "false")
	t.Setenv("SUZ_MOCK_ORDER_ID", "11111111-1111-4111-8111-111111111111")

	handler := newTestHandler()
	body := []byte(`{"productGroup":"mockpharma","products":[{"gtin":"00000000000000","quantity":20,"templateId":50}]}`)

	resp := performRequest(t, handler, http.MethodPost, "/api/v3/order?omsId=mock-oms-local", body, map[string]string{
		"Content-Type": "application/json",
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("create status = %d, want %d", resp.Code, http.StatusOK)
	}

	resp = performRequest(t, handler, http.MethodGet, "/__mock/state", nil, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("state status = %d, want %d", resp.Code, http.StatusOK)
	}

	var state struct {
		Orders  int `json:"orders"`
		Reports int `json:"reports"`
	}
	decodeJSON(t, resp.Body, &state)
	if state.Orders != 2 {
		t.Fatalf("orders = %d, want seeded order plus created order", state.Orders)
	}
	if state.Reports != 1 {
		t.Fatalf("reports = %d, want 1", state.Reports)
	}

	resp = performRequest(t, handler, http.MethodPost, "/__mock/reset", nil, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("reset status = %d, want %d", resp.Code, http.StatusOK)
	}

	resp = performRequest(t, handler, http.MethodGet, "/__mock/state", nil, nil)
	decodeJSON(t, resp.Body, &state)
	if state.Orders != 1 {
		t.Fatalf("orders after reset = %d, want seeded order", state.Orders)
	}
	if state.Reports != 1 {
		t.Fatalf("reports after reset = %d, want seeded report", state.Reports)
	}
}
