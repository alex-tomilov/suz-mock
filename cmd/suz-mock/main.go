package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultOMSId    = "cdf12109-10d3-11e6-8b6f-0050569977a1"
	defaultOrderID  = "b024ae09-ef7c-449e-b461-05d8eb116c79"
	defaultBlockID  = "012cc7b0-c9e4-4511-8058-2de1f97a87b0"
	defaultReportID = "fab1c0e4-9590-4ed7-8d58-18862d6a9aab"
	defaultGTIN     = "01334567894339"

	// API SUZ 3.0 scale-related limits from the PDF examples/notes.
	apiMaxProductPositionsPerOrder     = 10
	apiMaxSingleGTINOrderCodes         = 2_000_000
	apiMaxMultiGTINOrderCodes          = 150_000
	apiMaxReportCodes                  = 30_000
	apiMaxActiveOrders                 = 100
	apiMaxCreateOrderRequestsPerSecond = 100

	defaultHugeCodesPerResponse = apiMaxReportCodes
	defaultStreamFlushEvery     = 1000
)

type Server struct {
	mux     *http.ServeMux
	logger  *slog.Logger
	mu      sync.RWMutex
	orders  map[string]*Order
	reports map[string]*Report

	rateMu      sync.Mutex
	rateWindows map[string]*rateWindow
}

type rateWindow struct {
	unixSecond int64
	count      int
}

type Order struct {
	OMSId     string
	OrderID   string
	GTIN      string
	Quantity  int
	Status    string
	CreatedAt int64
	Closed    bool
}

type Report struct {
	OMSId     string
	ReportID  string
	Kind      string
	Status    string
	CreatedAt int64
}

type APIError struct {
	FieldErrors  []FieldError  `json:"fieldErrors"`
	GlobalErrors []GlobalError `json:"globalErrors"`
	Success      bool          `json:"success"`
}

type FieldError struct {
	FieldError string `json:"fieldError"`
	FieldName  string `json:"fieldName"`
	ErrorCode  int    `json:"errorCode"`
}

type GlobalError struct {
	Error     string `json:"error"`
	ErrorCode int    `json:"errorCode"`
}

func main() {
	addr := getenv("SUZ_MOCK_ADDR", ":8080")
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	srv := NewServer(logger)
	logger.Info("starting SUZ mock", "addr", addr)

	if err := http.ListenAndServe(addr, srv); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func NewServer(logger *slog.Logger) *Server {
	s := &Server{
		mux:         http.NewServeMux(),
		logger:      logger,
		orders:      make(map[string]*Order),
		reports:     make(map[string]*Report),
		rateWindows: make(map[string]*rateWindow),
	}

	s.seed()
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ww := &statusWriter{ResponseWriter: w, status: http.StatusOK}
	s.mux.ServeHTTP(ww, r)
	s.logger.Info("request",
		"method", r.Method,
		"path", r.URL.Path,
		"query", r.URL.RawQuery,
		"status", ww.status,
		"duration", time.Since(start).String(),
	)
}

func (s *Server) seed() {
	s.orders[defaultOrderID] = &Order{
		OMSId:     defaultOMSId,
		OrderID:   defaultOrderID,
		GTIN:      defaultGTIN,
		Quantity:  20,
		Status:    "READY",
		CreatedAt: 1550650989568,
	}
	s.reports[defaultReportID] = &Report{
		OMSId:     defaultOMSId,
		ReportID:  defaultReportID,
		Kind:      "aggregation",
		Status:    "SUCCESS",
		CreatedAt: time.Now().UnixMilli(),
	}
}

func (s *Server) routes() {
	s.mux.HandleFunc("/healthz", s.healthz)

	// API v3 routes based on the happy-path response examples from API SUZ 3.0.
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

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (s *Server) ping(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"omsId":      omsID(r),
		"apiVersion": "3.0.37-mock",
		"omsVersion": "5.0-mock",
	})
}

func (s *Server) createOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}

	var body struct {
		ProductGroup string `json:"productGroup"`
		Products     []struct {
			GTIN       string `json:"gtin"`
			Quantity   int    `json:"quantity"`
			TemplateID int    `json:"templateId"`
		} `json:"products"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	gtin := defaultString(r.URL.Query().Get("gtin"), defaultGTIN)
	quantity := intQuery(r, "quantity", 20)
	if len(body.Products) > 0 {
		gtin = defaultString(body.Products[0].GTIN, gtin)
		if body.Products[0].Quantity > 0 {
			quantity = body.Products[0].Quantity
		}
	}
	if scenario := strings.ToLower(r.URL.Query().Get("__scenario")); scenario == "huge" || scenario == "load" {
		quantity = intQuery(r, "quantity", defaultHugeCodesPerResponse)
	} else if scenario == "massive" {
		quantity = intQuery(r, "quantity", apiMaxMultiGTINOrderCodes)
	} else if scenario == "api_max" || scenario == "api-max" {
		quantity = intQuery(r, "quantity", apiMaxSingleGTINOrderCodes)
	}
	quantity = clampPositive(quantity, 1, intEnv("SUZ_MOCK_MAX_ORDER_QUANTITY", apiMaxSingleGTINOrderCodes))

	orderID := getenv("SUZ_MOCK_ORDER_ID", defaultOrderID)
	if boolEnv("SUZ_MOCK_DYNAMIC_IDS", false) {
		orderID = uuidLike()
	}

	order := &Order{
		OMSId:     omsID(r),
		OrderID:   orderID,
		GTIN:      gtin,
		Quantity:  quantity,
		Status:    "READY",
		CreatedAt: time.Now().UnixMilli(),
	}

	s.mu.Lock()
	s.orders[order.OrderID] = order
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]any{
		"omsId":                     order.OMSId,
		"orderId":                   order.OrderID,
		"expectedCompleteTimestamp": 5100,
	})
}
func (s *Server) orderStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}

	order := s.findOrder(r)
	scenario := strings.ToLower(defaultString(r.URL.Query().Get("__scenario"), "active"))
	gtin := defaultString(r.URL.Query().Get("gtin"), order.GTIN)

	switch scenario {
	case "rejected":
		writeJSON(w, http.StatusOK, []map[string]any{{
			"omsId":            order.OMSId,
			"orderId":          order.OrderID,
			"leftInBuffer":     -1,
			"poolsExhausted":   false,
			"totalCodes":       -1,
			"unavailableCodes": -1,
			"availableCodes":   -1,
			"gtin":             gtin,
			"bufferStatus":     "REJECTED",
			"rejectionReason":  "Order declined: mock validation failed",
			"totalPassed":      -1,
			"templateId":       50,
		}})
	case "pending":
		writeJSON(w, http.StatusOK, []map[string]any{{
			"omsId":            order.OMSId,
			"orderId":          order.OrderID,
			"leftInBuffer":     -1,
			"poolsExhausted":   false,
			"totalCodes":       -1,
			"unavailableCodes": -1,
			"availableCodes":   -1,
			"gtin":             gtin,
			"bufferStatus":     "PENDING",
			"totalPassed":      -1,
			"templateId":       50,
		}})
	default:
		writeJSON(w, http.StatusOK, []map[string]any{{
			"omsId":            order.OMSId,
			"orderId":          order.OrderID,
			"leftInBuffer":     0,
			"totalCodes":       order.Quantity,
			"poolsExhausted":   false,
			"unavailableCodes": 0,
			"availableCodes":   order.Quantity,
			"gtin":             gtin,
			"bufferStatus":     "ACTIVE",
			"totalPassed":      0,
			"expiredDate":      "1596792681987",
			"templateId":       50,
		}})
	}
}

func (s *Server) orderList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}

	requestedOrders := intQuery(r, "__orders", 0)
	if strings.EqualFold(r.URL.Query().Get("__scenario"), "huge") && requestedOrders == 0 {
		requestedOrders = apiMaxActiveOrders
	}

	if requestedOrders > 0 {
		requestedOrders = clampPositive(requestedOrders, 1, intEnv("SUZ_MOCK_MAX_ORDER_LIST", apiMaxActiveOrders))
		writeJSON(w, http.StatusOK, map[string]any{
			"omsId":      omsID(r),
			"orderInfos": makeOrderInfos(requestedOrders, intQuery(r, "__buffers", 1), intQuery(r, "quantity", defaultHugeCodesPerResponse)),
		})
		return
	}

	s.mu.RLock()
	infos := make([]map[string]any, 0, len(s.orders))
	for _, o := range s.orders {
		status := o.Status
		if o.Closed {
			status = "CLOSED"
		}
		infos = append(infos, map[string]any{
			"orderId":          o.OrderID,
			"orderStatus":      status,
			"createdTimestamp": o.CreatedAt,
			"productGroup":     "vetpharma",
			"buffers": []map[string]any{{
				"leftInBuffer":     o.Quantity,
				"totalCodes":       o.Quantity,
				"unavailableCodes": 0,
				"availableCodes":   o.Quantity,
				"gtin":             o.GTIN,
				"bufferStatus":     "ACTIVE",
				"templateId":       50,
			}},
		})
	}
	s.mu.RUnlock()

	writeJSON(w, http.StatusOK, map[string]any{
		"omsId":      omsID(r),
		"orderInfos": infos,
	})
}
func (s *Server) getCodes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}

	quantity := intQuery(r, "quantity", 1)
	scenario := strings.ToLower(r.URL.Query().Get("__scenario"))
	switch scenario {
	case "huge", "load":
		quantity = intQuery(r, "quantity", defaultHugeCodesPerResponse)
	case "massive":
		quantity = intQuery(r, "quantity", apiMaxMultiGTINOrderCodes)
	case "api_max", "api-max":
		quantity = intQuery(r, "quantity", apiMaxSingleGTINOrderCodes)
	}
	quantity = clampPositive(quantity, 1, intEnv("SUZ_MOCK_MAX_CODES_PER_RESPONSE", defaultHugeCodesPerResponse))

	gtin := defaultString(r.URL.Query().Get("gtin"), defaultGTIN)
	blockID := defaultString(r.URL.Query().Get("lastBlockId"), defaultBlockID)

	if quantity > intEnv("SUZ_MOCK_STREAM_THRESHOLD", 1000) || truthy(r.URL.Query().Get("__stream")) {
		writeCodesJSON(w, r, omsID(r), gtin, quantity, blockID)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"omsId":   omsID(r),
		"codes":   makeCodes(gtin, quantity),
		"blockId": blockID,
	})
}
func (s *Server) codeBlocks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}

	blockCount := intQuery(r, "__blocks", 2)
	if strings.EqualFold(r.URL.Query().Get("__scenario"), "huge") {
		blockCount = intQuery(r, "__blocks", 100)
	}
	blockCount = clampPositive(blockCount, 1, intEnv("SUZ_MOCK_MAX_BLOCKS", 1000))
	blockQuantity := clampPositive(intQuery(r, "__blockQuantity", 100), 1, apiMaxSingleGTINOrderCodes)

	blocks := make([]map[string]any, 0, blockCount)
	for i := 0; i < blockCount; i++ {
		blocks = append(blocks, map[string]any{
			"blockId":       fmt.Sprintf("a024ae09-ef7c-449e-b461-%012d", i+1),
			"blockDateTime": 1573986891 + i,
			"quantity":      blockQuantity,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"orderId": defaultString(r.URL.Query().Get("orderId"), defaultOrderID),
		"omsId":   omsID(r),
		"gtin":    defaultString(r.URL.Query().Get("gtin"), defaultGTIN),
		"blocks":  blocks,
	})
}
func (s *Server) retryCodes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	quantity := clampPositive(intQuery(r, "quantity", 1), 1, intEnv("SUZ_MOCK_MAX_CODES_PER_RESPONSE", defaultHugeCodesPerResponse))
	blockID := defaultString(r.URL.Query().Get("blockId"), "a024ae09-ef7c-449e-b461-05d8eb116c90")
	if quantity > intEnv("SUZ_MOCK_STREAM_THRESHOLD", 1000) || truthy(r.URL.Query().Get("__stream")) {
		writeCodesJSON(w, r, omsID(r), defaultGTIN, quantity, blockID)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"omsId":   omsID(r),
		"codes":   makeCodes(defaultGTIN, quantity),
		"blockId": blockID,
	})
}
func (s *Server) productAttributes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if r.URL.Query().Get("__scenario") == "empty" {
		writeJSON(w, http.StatusOK, map[string]any{})
		return
	}

	productCount := intQuery(r, "__products", 1)
	if strings.EqualFold(r.URL.Query().Get("__scenario"), "huge") {
		productCount = intQuery(r, "__products", apiMaxProductPositionsPerOrder)
	}
	productCount = clampPositive(productCount, 1, intEnv("SUZ_MOCK_MAX_PRODUCTS", apiMaxProductPositionsPerOrder))

	attrs := make(map[string]any, productCount)
	for i := 0; i < productCount; i++ {
		gtin := fmt.Sprintf("0133456789%04d", 4339+i)
		attrs[gtin] = map[string]any{
			"fat":                      "2",
			"name":                     fmt.Sprintf("Regress SUZ Mock Product %d", i+1),
			"brand":                    "crpt",
			"fullName":                 fmt.Sprintf("Regress SUZ Mock Product %d", i+1),
			"tnVedCode":                "0402",
			"packageType":              "НЕ УКАЗАН",
			"tnVedCode10":              "0402291500",
			"paymentGroup":             3,
			"productGroup":             8,
			"volumeWeight":             "100 мл",
			"babyFoodProduct":          "ДА (ОТ 3 ЛЕТ)",
			"milkProductType":          "сыр",
			"isShelfLife40Days":        "ДА",
			"veterinaryControl":        "НЕТ",
			"isSpecializedFoodProduct": "ДА",
		}
	}
	writeJSON(w, http.StatusOK, attrs)
}
func (s *Server) closeOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	var body struct {
		OrderID string `json:"orderId"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.OrderID == "" {
		body.OrderID = defaultOrderID
	}

	s.mu.Lock()
	if order, ok := s.orders[body.OrderID]; ok {
		order.Closed = true
	}
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]any{"omsId": omsID(r)})
}

func (s *Server) acceptReport(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}
		_, _ = io.Copy(io.Discard, r.Body)

		reportID := defaultReportID
		if getenv("SUZ_MOCK_DYNAMIC_IDS", "false") == "true" {
			reportID = uuidLike()
		}
		report := &Report{
			OMSId:     omsID(r),
			ReportID:  reportID,
			Kind:      kind,
			Status:    "SUCCESS",
			CreatedAt: time.Now().UnixMilli(),
		}

		s.mu.Lock()
		s.reports[report.ReportID] = report
		s.mu.Unlock()

		writeJSON(w, http.StatusOK, map[string]any{
			"omsId":    report.OMSId,
			"reportId": report.ReportID,
		})
	}
}

func (s *Server) reportInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	reportID := defaultString(r.URL.Query().Get("reportId"), defaultReportID)
	status := "SUCCESS"

	s.mu.RLock()
	if report, ok := s.reports[reportID]; ok {
		status = report.Status
	}
	s.mu.RUnlock()

	if override := r.URL.Query().Get("__scenario"); override != "" {
		status = strings.ToUpper(override)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"omsId":        omsID(r),
		"reportId":     reportID,
		"reportStatus": status,
	})
}

func (s *Server) providers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"providers": []map[string]any{{
			"serviceProviderId":       "a5ed4f3d-150b-49ae-bc1d-1582c4da634a",
			"providerName":            "ООО \"Лориполь\"",
			"name":                    "CM 6093 ООО \"Лориполь\"",
			"taxIdentificationNumber": "5835039864",
			"country":                 "RU",
			"address":                 "г.Москва Кашширское шоссе д.12",
			"contactPerson":           "Иванов",
			"email":                   "ivanov@mail.ru",
			"productGroups":           []string{"lp", "otp", "milk", "electronics", "tires", "perfumery", "water"},
		}},
	})
}

func (s *Server) documentContent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	items := intQuery(r, "__items", 2)
	if strings.EqualFold(r.URL.Query().Get("__scenario"), "huge") {
		items = intQuery(r, "__items", apiMaxReportCodes)
	}
	items = clampPositive(items, 1, intEnv("SUZ_MOCK_MAX_DOCUMENT_ITEMS", apiMaxReportCodes))

	cislist := make([]map[string]any, 0, items)
	for i := 0; i < items; i++ {
		cislist = append(cislist, map[string]any{
			"cis":  fmt.Sprintf("0104616052543035215MOCK%09d", i+1),
			"code": 2,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"content": map[string]any{
			"cislist": cislist,
		},
	})
}
func (s *Server) findOrder(r *http.Request) *Order {
	orderID := defaultString(r.URL.Query().Get("orderId"), defaultOrderID)

	s.mu.RLock()
	defer s.mu.RUnlock()
	if order, ok := s.orders[orderID]; ok {
		return order
	}
	return &Order{
		OMSId:     omsID(r),
		OrderID:   orderID,
		GTIN:      defaultString(r.URL.Query().Get("gtin"), defaultGTIN),
		Quantity:  20,
		Status:    "READY",
		CreatedAt: time.Now().UnixMilli(),
	}
}

func (s *Server) allowRequest(r *http.Request, limit int) bool {
	if limit <= 0 {
		return true
	}
	key := r.RemoteAddr + "|" + omsID(r) + "|" + r.URL.Path
	if forwardedFor := r.Header.Get("X-Forwarded-For"); forwardedFor != "" {
		key = strings.TrimSpace(strings.Split(forwardedFor, ",")[0]) + "|" + omsID(r) + "|" + r.URL.Path
	}
	now := time.Now().Unix()

	s.rateMu.Lock()
	defer s.rateMu.Unlock()
	win := s.rateWindows[key]
	if win == nil || win.unixSecond != now {
		s.rateWindows[key] = &rateWindow{unixSecond: now, count: 1}
		return true
	}
	win.count++
	return win.count <= limit
}

func makeOrderInfos(orderCount, bufferCount, quantity int) []map[string]any {
	bufferCount = clampPositive(bufferCount, 1, apiMaxProductPositionsPerOrder)
	quantity = clampPositive(quantity, 1, apiMaxSingleGTINOrderCodes)
	infos := make([]map[string]any, 0, orderCount)
	for i := 0; i < orderCount; i++ {
		buffers := make([]map[string]any, 0, bufferCount)
		for j := 0; j < bufferCount; j++ {
			buffers = append(buffers, map[string]any{
				"leftInBuffer":     quantity,
				"totalCodes":       quantity,
				"unavailableCodes": 0,
				"availableCodes":   quantity,
				"gtin":             fmt.Sprintf("0133456789%04d", 4339+j),
				"bufferStatus":     "ACTIVE",
				"templateId":       50,
			})
		}
		infos = append(infos, map[string]any{
			"orderId":          fmt.Sprintf("b024ae09-ef7c-449e-b461-%012d", i+1),
			"orderStatus":      "READY",
			"createdTimestamp": time.Now().Add(-time.Duration(i) * time.Minute).UnixMilli(),
			"productGroup":     "vetpharma",
			"buffers":          buffers,
		})
	}
	return infos
}

func writeCodesJSON(w http.ResponseWriter, r *http.Request, omsIDValue, gtin string, quantity int, blockID string) {
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, `{"omsId":`)
	writeJSONString(w, omsIDValue)
	_, _ = io.WriteString(w, `,"codes":[`)

	flushEvery := clampPositive(intQuery(r, "__flushEvery", intEnv("SUZ_MOCK_STREAM_FLUSH_EVERY", defaultStreamFlushEvery)), 1, quantity)
	chunkDelay := durationQuery(r, "__chunkDelay", 0)
	flusher, _ := w.(http.Flusher)

	for i := 0; i < quantity; i++ {
		if i > 0 {
			_, _ = io.WriteString(w, ",")
		}
		writeJSONString(w, makeCode(gtin, i))
		if flusher != nil && (i+1)%flushEvery == 0 {
			flusher.Flush()
			if chunkDelay > 0 {
				time.Sleep(chunkDelay)
			}
		}
	}
	_, _ = io.WriteString(w, `],"blockId":`)
	writeJSONString(w, blockID)
	_, _ = io.WriteString(w, `}`+"\n")
}

func writeJSONString(w io.Writer, value string) {
	encoded, _ := json.Marshal(value)
	_, _ = w.Write(encoded)
}

func setCommonHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json;charset=UTF-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, max-age=0, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-XSS-Protection", "1; mode=block")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-RequestId", uuidLike())
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(value)
}

func writeError(w http.ResponseWriter, status int, msg string, code int) {
	writeJSON(w, status, APIError{
		FieldErrors: []FieldError{},
		GlobalErrors: []GlobalError{{
			Error:     msg,
			ErrorCode: code,
		}},
		Success: false,
	})
}

func methodNotAllowed(w http.ResponseWriter, allowed string) {
	w.Header().Set("Allow", allowed)
	writeError(w, http.StatusMethodNotAllowed, "method not allowed", http.StatusMethodNotAllowed)
}

func omsID(r *http.Request) string {
	return defaultString(r.URL.Query().Get("omsId"), defaultOMSId)
}

func intQuery(r *http.Request, key string, fallback int) int {
	value := r.URL.Query().Get(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func clampPositive(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if maxValue > 0 && value > maxValue {
		return maxValue
	}
	return value
}

func intEnv(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func boolEnv(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return truthy(value)
}

func truthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func durationQuery(r *http.Request, key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return fallback
	}
	if d, err := time.ParseDuration(value); err == nil {
		return d
	}
	millis, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return time.Duration(millis) * time.Millisecond
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func makeCodes(gtin string, quantity int) []string {
	codes := make([]string, 0, quantity)
	for i := 0; i < quantity; i++ {
		codes = append(codes, makeCode(gtin, i))
	}
	return codes
}

func makeCode(gtin string, index int) string {
	serial := fmt.Sprintf("MOCK%09d", index+1)
	// GS1 group separator is ASCII 29. json.Encoder serializes it as \u001d.
	return fmt.Sprintf("01%s21%s%c93VXQI", gtin, serial, rune(29))
}

func uuidLike() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		panic(errors.Join(errors.New("cannot generate uuid"), err))
	}
	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(buf[0:4]),
		hex.EncodeToString(buf[4:6]),
		hex.EncodeToString(buf[6:8]),
		hex.EncodeToString(buf[8:10]),
		hex.EncodeToString(buf[10:16]),
	)
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
