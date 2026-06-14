package httputil

import (
	"encoding/json"
	"net/http"
)

type ErrorBody struct {
	Msg string `json:"msg"`
}

func WriteJSON(w http.ResponseWriter, status int, v any) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(v)
}

func WriteError(w http.ResponseWriter, status int, message string) {
	_ = WriteJSON(w, status, ErrorBody{Msg: message})
}
