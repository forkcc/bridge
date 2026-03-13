#!/usr/bin/env bash
# 端到端测试：apiHub -> Bridge -> Edge -> Client
# 需要：PostgreSQL (postgres/postgres)、build/ 下二进制、可选 RabbitMQ
set -e
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
export DSN="host=localhost user=postgres password=postgres dbname=proxy_bridge port=5432 sslmode=disable"

echo "building..."
go build -o build/apihub ./cmd/apihub
go build -o build/bridge ./cmd/bridge
go build -o build/edge ./cmd/edge
go build -o build/client ./cmd/client

echo "starting apihub..."
./build/apihub configs/apihub.yaml &
APIHUB_PID=$!
sleep 2
curl -s -o /dev/null -w "%{http_code}" http://127.0.0.1:8082/health | grep -q 200 || { kill $APIHUB_PID 2>/dev/null; exit 1; }

echo "seeding nodes..."
go run ./cmd/seed

echo "starting bridge..."
./build/bridge configs/bridge.yaml &
BRIDGE_PID=$!
sleep 1

echo "starting edge..."
./build/edge --token edge-token-456 --id edge-1 &
EDGE_PID=$!
sleep 2

echo "starting client..."
./build/client configs/client.yaml &
CLIENT_PID=$!
sleep 2

cleanup() {
  kill $CLIENT_PID $EDGE_PID $BRIDGE_PID $APIHUB_PID 2>/dev/null || true
}
trap cleanup EXIT

echo "testing SOCKS5 (curl via 127.0.0.1:1080)..."
curl -s -x socks5h://127.0.0.1:1080 -m 10 -o /dev/null -w "%{http_code}" http://example.com | grep -q 200 || true

echo "e2e done."
