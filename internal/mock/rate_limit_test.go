package mock

import (
	"net/http"
	"testing"
)

func TestRateLimitScenarioReturns429(t *testing.T) {
	t.Setenv("SUZ_MOCK_RATE_LIMIT_PER_SECOND", "1")

	handler := newTestHandler()
	target := "/api/v3/ping?omsId=mock-oms-local&__rateLimit=1"

	first := performRequest(t, handler, http.MethodGet, target, nil, map[string]string{
		"X-Forwarded-For": "203.0.113.10",
	})
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, want %d", first.Code, http.StatusOK)
	}

	second := performRequest(t, handler, http.MethodGet, target, nil, map[string]string{
		"X-Forwarded-For": "203.0.113.10",
	})
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d, want %d", second.Code, http.StatusTooManyRequests)
	}

	var apiErr APIError
	decodeJSON(t, second.Body, &apiErr)

	if apiErr.Success {
		t.Fatal("success = true, want false")
	}
	if len(apiErr.GlobalErrors) != 1 {
		t.Fatalf("globalErrors len = %d, want 1", len(apiErr.GlobalErrors))
	}
	if apiErr.GlobalErrors[0].ErrorCode != http.StatusTooManyRequests {
		t.Fatalf("errorCode = %d, want %d", apiErr.GlobalErrors[0].ErrorCode, http.StatusTooManyRequests)
	}
}
