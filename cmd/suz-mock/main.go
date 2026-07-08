package main

import (
	"log"

	"github.com/alex-tomilov/suz-mock/internal/mock"
)

func main() {
	cfg := mock.LoadConfigFromEnv()
	srv := mock.NewServer(cfg)

	cfg.Logger.Info("starting SUZ mock", "addr", srv.Addr)

	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
