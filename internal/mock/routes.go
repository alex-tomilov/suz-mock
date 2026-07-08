package mock

import (
	"fmt"
	"net/http"
	"strconv"
	"time"
)

func (s *Server) routes() {
	s.mux.HandleFunc("/healthz", s.healthz)

	// API v3 routes for stable local client-contract testing.
	s.mux.HandleFunc("/api/v3/ping", s.withCommon(s.ping))
	s.mux.HandleFunc("/api/v3/order", s.withCommon(s.createOrder))
	s.mux.HandleFunc("/api/v3/order/status", s.withCommon(s.orderStatus))
	s.mux.HandleFunc("/api/v3/order/list", s.withCommon(s.orderList))
	s.mux.HandleFunc("/api/v3/codes", s.withCommon(s.getCodes))
	s.mux.HandleFunc("/api/v3/order/codes/blocks", s.withCommon(s.codeBlocks))
	s.mux.HandleFunc("/api/v3/order/codes/retry", s.withCommon(s.retryCodes))
	s.mux.HandleFunc("/api/v3/order/product", s.withCommon(s.productAttributes))
	s.mux.HandleFunc("/api/v3/order/close", s.withCommon(s.closeOrder))

	s.mux.HandleFunc("/api/v3/dropout", s.withCommon(s.acceptReport("dropout")))
	s.mux.HandleFunc("/api/v3/aggregation", s.withCommon(s.acceptReport("aggregation")))
	s.mux.HandleFunc("/api/v3/utilisation", s.withCommon(s.acceptReport("utilisation")))
	s.mux.HandleFunc("/api/v3/surplus", s.withCommon(s.acceptReport("surplus")))
	s.mux.HandleFunc("/api/v3/report/info", s.withCommon(s.reportInfo))
	s.mux.HandleFunc("/api/v3/providers", s.withCommon(s.providers))
	s.mux.HandleFunc("/api/v3/documents/content", s.withCommon(s.documentContent))
}

func (s *Server) withCommon(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setCommonHeaders(w)

		if delay := durationQuery(r, "__delay", 0); delay > 0 {
			time.Sleep(delay)
		}

		if code := r.URL.Query().Get("__error"); code != "" {
			status, err := strconv.Atoi(code)
			if err != nil || status < 400 || status > 599 {
				status = http.StatusBadRequest
			}
			writeError(w, status, fmt.Sprintf("forced mock error %d", status), status)
			return
		}

		if boolEnv("SUZ_MOCK_REQUIRE_TOKEN", false) {
			if r.Header.Get("clientToken") == "" && r.Header.Get("Authorization") == "" {
				writeError(w, http.StatusUnauthorized, "missing clientToken or Authorization header", 400)
				return
			}
		}

		if boolEnv("SUZ_MOCK_RATE_LIMIT", false) || truthy(r.URL.Query().Get("__rateLimit")) {
			limit := intEnv("SUZ_MOCK_RATE_LIMIT_PER_SECOND", apiMaxCreateOrderRequestsPerSecond)
			if !s.allowRequest(r, limit) {
				w.Header().Set("Retry-After", "1")
				writeError(w, http.StatusTooManyRequests, "mock rate limit exceeded", http.StatusTooManyRequests)
				return
			}
		}

		next(w, r)
	}
}
