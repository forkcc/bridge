#!/usr/bin/env bash
# 端到端测试：apiHub -> Bridge -> Edge -> Client
# 需要：PostgreSQL (postgres/postgres)、build/ 下二进制
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
./build/edge --token e2e-token --id edge-1 &
EDGE_PID=$!
sleep 2

echo "starting client..."
./build/client --token e2e-token --country cn --config configs/client.yaml &
CLIENT_PID=$!
sleep 2

cleanup() {
  kill $CLIENT_PID $EDGE_PID $BRIDGE_PID $APIHUB_PID 2>/dev/null || true
}
trap cleanup EXIT

echo "testing SOCKS5: 100+ requests required..."
SUCCESS=0
TOTAL=110
for i in $(seq 1 $TOTAL); do
  CODE=$(curl -s -x socks5h://127.0.0.1:1080 -m 15 -o /dev/null -w "%{http_code}" http://example.com 2>/dev/null || echo "000")
  if [ "$CODE" = "200" ]; then
    SUCCESS=$((SUCCESS + 1))
  fi
  if [ $((i % 20)) -eq 0 ]; then
    echo "  $i/$TOTAL ok=$SUCCESS"
  fi
done
echo "result: $SUCCESS/$TOTAL"
if [ "$SUCCESS" -lt 100 ]; then
  echo "e2e FAIL: need 100+ success, got $SUCCESS"
  exit 1
fi
echo "e2e PASS: $SUCCESS requests succeeded."
