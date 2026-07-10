package mock

import (
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultOMSId    = "cdf12109-10d3-11e6-8b6f-0050569977a1"
	defaultOrderID  = "b024ae09-ef7c-449e-b461-05d8eb116c79"
	defaultBlockID  = "012cc7b0-c9e4-4511-8058-2de1f97a87b0"
	defaultReportID = "fab1c0e4-9590-4ed7-8d58-18862d6a9aab"
	defaultGTIN     = "01334567894339"

	apiMaxProductPositionsPerOrder     = 10
	apiMaxSingleGTINOrderCodes         = 2_000_000
	apiMaxMultiGTINOrderCodes          = 150_000
	apiMaxReportCodes                  = 30_000
	apiMaxActiveOrders                 = 100
	apiMaxCreateOrderRequestsPerSecond = 100

	defaultHugeCodesPerResponse = apiMaxReportCodes
	defaultStreamFlushEvery     = 1000
)

type Config struct {
	Addr      string
	Logger    *slog.Logger
	LogLevel  slog.Level
	AccessLog bool
	JSONLogs  bool
}

func LoadConfigFromEnv() Config {
	level := parseLogLevel(getenv("SUZ_MOCK_LOG_LEVEL", "info"))
	jsonLogs := boolEnv("SUZ_MOCK_JSON_LOGS", false)

	return Config{
		Addr:      getenv("SUZ_MOCK_ADDR", ":8080"),
		Logger:    newLogger(os.Stdout, level, jsonLogs),
		LogLevel:  level,
		AccessLog: boolEnv("SUZ_MOCK_ACCESS_LOG", true),
		JSONLogs:  jsonLogs,
	}
}

func newLogger(out io.Writer, level slog.Level, jsonLogs bool) *slog.Logger {
	opts := &slog.HandlerOptions{Level: level}
	if jsonLogs {
		return slog.New(slog.NewJSONHandler(out, opts))
	}
	return slog.New(slog.NewTextHandler(out, opts))
}

func parseLogLevel(value string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
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
