package mock

import "net/http"

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
