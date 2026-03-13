package main

import (
	"log"
	"os"

	"proxy-bridge/internal/bridge"
	"proxy-bridge/pkg/config"
)

func main() {
	cfgPath := "configs/bridge.yaml"
	if len(os.Args) > 1 {
		cfgPath = os.Args[1]
	}
	var cfg bridge.Config
	if err := config.Load(cfgPath, &cfg); err != nil {
		log.Fatalf("load config: %v", err)
	}
	srv := bridge.NewServer(&cfg)
	if err := srv.Run(); err != nil {
		log.Fatalf("run: %v", err)
	}
}
