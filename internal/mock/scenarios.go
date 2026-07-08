package mock

import (
	"net/http"
	"strings"
)

func orderQuantityForScenario(r *http.Request, fallback int) int {
	switch scenarioName(r) {
	case "huge", "load":
		return intQuery(r, "quantity", defaultHugeCodesPerResponse)
	case "massive":
		return intQuery(r, "quantity", apiMaxMultiGTINOrderCodes)
	case "api_max", "api-max":
		return intQuery(r, "quantity", apiMaxSingleGTINOrderCodes)
	default:
		return fallback
	}
}

func codeQuantityForScenario(r *http.Request, fallback int) int {
	switch scenarioName(r) {
	case "huge", "load":
		return intQuery(r, "quantity", defaultHugeCodesPerResponse)
	case "massive":
		return intQuery(r, "quantity", apiMaxMultiGTINOrderCodes)
	case "api_max", "api-max":
		return intQuery(r, "quantity", apiMaxSingleGTINOrderCodes)
	default:
		return fallback
	}
}

func orderListCountForScenario(r *http.Request) int {
	requestedOrders := intQuery(r, "__orders", 0)
	if scenarioName(r) == "huge" && requestedOrders == 0 {
		return apiMaxActiveOrders
	}
	return requestedOrders
}

func blockCountForScenario(r *http.Request) int {
	if scenarioName(r) == "huge" {
		return intQuery(r, "__blocks", 100)
	}
	return intQuery(r, "__blocks", 2)
}

func productCountForScenario(r *http.Request) int {
	if scenarioName(r) == "huge" {
		return intQuery(r, "__products", apiMaxProductPositionsPerOrder)
	}
	return intQuery(r, "__products", 1)
}

func documentItemCountForScenario(r *http.Request) int {
	if scenarioName(r) == "huge" {
		return intQuery(r, "__items", apiMaxReportCodes)
	}
	return intQuery(r, "__items", 2)
}

func scenarioName(r *http.Request) string {
	return strings.ToLower(r.URL.Query().Get("__scenario"))
}
