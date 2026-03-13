package main

import (
	"flag"
	"log"
	"os"

	"proxy-bridge/internal/client"
	"proxy-bridge/pkg/config"
)

func main() {
	token := flag.String("token", "", "client token (required)")
	id := flag.String("id", "", "client id (optional)")
	country := flag.String("country", "", "country code for edge matching (e.g. cn, us)")
	cfgPath := flag.String("config", "", "config file path (optional)")
	flag.Usage = func() {
		log.Printf("usage: client --token TOKEN [--country CODE] [--id CLIENT_ID] [--config path]")
		flag.PrintDefaults()
	}
	flag.Parse()
	if *token == "" {
		flag.Usage()
		os.Exit(1)
	}
	var cfg client.Config
	if *cfgPath != "" {
		_ = config.Load(*cfgPath, &cfg)
	}
	cfg.Token = *token
	if *id != "" {
		cfg.ID = *id
	}
	if *country != "" {
		cfg.Country = *country
	}
	if cfg.Socks5Listen == "" {
		cfg.Socks5Listen = ":1080"
	}
	if cfg.BridgeURL == "" {
		cfg.BridgeURL = "http://localhost:8080"
	}
	if cfg.BridgeTunnel == "" {
		cfg.BridgeTunnel = "localhost:8081"
	}
	srv := client.NewServer(&cfg)
	if err := srv.Run(); err != nil {
		log.Fatalf("run: %v", err)
	}
}
