package main

import (
	"log"
	"os"

	"proxy-bridge/internal/apihub"
	"proxy-bridge/pkg/config"
)

func main() {
	cfgPath := "configs/apihub.yaml"
	if len(os.Args) > 1 {
		cfgPath = os.Args[1]
	}
	var cfg apihub.Config
	if err := config.Load(cfgPath, &cfg); err != nil {
		log.Fatalf("load config: %v", err)
	}
	srv, err := apihub.NewServer(&cfg)
	if err != nil {
		log.Fatalf("new server: %v", err)
	}
	if err := srv.Run(); err != nil {
		log.Fatalf("run: %v", err)
	}
}
