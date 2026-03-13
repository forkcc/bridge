package main

import (
	"flag"
	"log"
	"os"

	"proxy-bridge/internal/edge"
	"proxy-bridge/pkg/config"
)

func main() {
	token := flag.String("token", "", "edge token (required)")
	id := flag.String("id", "", "edge id (required)")
	cfgPath := flag.String("config", "configs/edge.yaml", "config file path")
	flag.Usage = func() {
		log.Printf("usage: edge --token TOKEN --id EDGE_ID [--config path]")
		flag.PrintDefaults()
	}
	flag.Parse()
	if *token == "" || *id == "" {
		flag.Usage()
		os.Exit(1)
	}
	var cfg edge.Config
	if err := config.Load(*cfgPath, &cfg); err != nil {
		log.Fatalf("load config: %v", err)
	}
	cfg.Token = *token
	cfg.ID = *id
	if cfg.Listen == "" {
		cfg.Listen = ":60001"
	}
	if cfg.ApihubURL == "" {
		cfg.ApihubURL = "http://localhost:8082"
	}
	srv := edge.NewServer(&cfg)
	if err := srv.Run(); err != nil {
		log.Fatalf("run: %v", err)
	}
}
