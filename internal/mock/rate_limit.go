package mock

import (
	"net/http"
	"strings"
	"time"
)

type rateWindow struct {
	unixSecond int64
	count      int
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
