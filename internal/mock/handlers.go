package mock

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

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
	quantity = orderQuantityForScenario(r, quantity)
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

	requestedOrders := orderListCountForScenario(r)
	if requestedOrders > 0 {
		requestedOrders = clampPositive(requestedOrders, 1, intEnv("SUZ_MOCK_MAX_ORDER_LIST", apiMaxActiveOrders))
		writeJSON(w, http.StatusOK, map[string]any{
			"omsId": omsID(r),
			"orderInfos": makeOrderInfos(
				requestedOrders,
				intQuery(r, "__buffers", 1),
				intQuery(r, "quantity", defaultHugeCodesPerResponse),
			),
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

	quantity := codeQuantityForScenario(r, intQuery(r, "quantity", 1))
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

	blockCount := blockCountForScenario(r)
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

	productCount := productCountForScenario(r)
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
	items := documentItemCountForScenario(r)
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
