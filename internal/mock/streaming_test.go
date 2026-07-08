package mock

import (
	"net/http"
	"testing"
)

func TestCodesQuantity30000ReturnsValidJSON(t *testing.T) {
	t.Setenv("SUZ_MOCK_MAX_CODES_PER_RESPONSE", "30000")

	handler := newTestHandler()

	resp := performRequest(t, handler, http.MethodGet, "/api/v3/codes?omsId=mock-oms-local&gtin=00000000000000&quantity=30000", nil, nil)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
	}

	var body codesResponse
	decodeJSON(t, resp.Body, &body)

	if body.OMSId != "mock-oms-local" {
		t.Fatalf("omsId = %q, want %q", body.OMSId, "mock-oms-local")
	}
	if len(body.Codes) != 30000 {
		t.Fatalf("codes len = %d, want 30000", len(body.Codes))
	}
	if body.BlockID == "" {
		t.Fatal("blockId is empty")
	}
}

func TestCodesQuantityCapIsRespected(t *testing.T) {
	t.Setenv("SUZ_MOCK_MAX_CODES_PER_RESPONSE", "3")

	handler := newTestHandler()

	resp := performRequest(t, handler, http.MethodGet, "/api/v3/codes?omsId=mock-oms-local&gtin=00000000000000&quantity=10", nil, nil)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
	}

	var body codesResponse
	decodeJSON(t, resp.Body, &body)

	if len(body.Codes) != 3 {
		t.Fatalf("codes len = %d, want 3", len(body.Codes))
	}
}

func TestSlowStreamingProducesValidJSON(t *testing.T) {
	handler := newTestHandler()

	resp := performRequest(t, handler, http.MethodGet, "/api/v3/codes?omsId=mock-oms-local&gtin=00000000000000&quantity=5&__stream=1&__flushEvery=1&__chunkDelay=1ms", nil, nil)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
	}

	var body codesResponse
	decodeJSON(t, resp.Body, &body)

	if len(body.Codes) != 5 {
		t.Fatalf("codes len = %d, want 5", len(body.Codes))
	}
}

type codesResponse struct {
	OMSId   string   `json:"omsId"`
	Codes   []string `json:"codes"`
	BlockID string   `json:"blockId"`
}
