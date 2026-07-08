package mock

import (
	"io"
	"net/http"
	"time"
)

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
