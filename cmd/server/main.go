package main

import (
	"log"
	"net/http"
	"time"

	"test-auto-pro-v2/internal/api"
	"test-auto-pro-v2/internal/config"
)

func main() {
	server := &http.Server{
		Addr:              config.ServerAddress(),
		Handler:           api.NewHandler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("后端服务监听 %s", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
