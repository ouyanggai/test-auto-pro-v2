package api

import (
	"net/http"
)

const healthResponse = `{"status":"ok","service":"test-auto-pro","version":"dev"}`

func NewHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", health)
	return mux
}

func health(response http.ResponseWriter, _ *http.Request) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(http.StatusOK)
	_, _ = response.Write([]byte(healthResponse))
}
