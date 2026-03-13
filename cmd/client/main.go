package main

import (
	"log"
	"os"

	"proxy-bridge/internal/client"
	"proxy-bridge/pkg/config"
)

func main() {
	cfgPath := "configs/client.yaml"
	if len(os.Args) > 1 {
		cfgPath = os.Args[1]
	}
	var cfg client.Config
	if err := config.Load(cfgPath, &cfg); err != nil {
		log.Fatalf("load config: %v", err)
	}
	srv := client.NewServer(&cfg)
	if err := srv.Run(); err != nil {
		log.Fatalf("run: %v", err)
	}
}
