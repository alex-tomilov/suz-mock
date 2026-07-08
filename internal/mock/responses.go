package mock

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

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

func writeJSONString(w io.Writer, value string) {
	encoded, _ := json.Marshal(value)
	_, _ = w.Write(encoded)
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
