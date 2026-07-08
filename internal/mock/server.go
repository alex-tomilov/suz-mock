package mock

import (
	"log/slog"
	"net/http"
	"sync"
	"time"
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

func NewServer(cfg Config) *http.Server {
	addr := defaultString(cfg.Addr, ":8080")
	handler := newHandler(cfg.Logger)

	return &http.Server{
		Addr:    addr,
		Handler: handler,
	}
}

func newHandler(logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}

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
