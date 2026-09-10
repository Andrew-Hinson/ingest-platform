#!/usr/bin/env bash
# Forwards local Kafka cluster access from k8s: kills any stale listeners on the
# target ports, then port-forwards the 3 dual-role broker pods (19092-19094),
# Karapace schema registry (8081), Prometheus (9090), and Grafana (3000).
# Ctrl-C tears down all forwards via the EXIT trap.
set -euo pipefail
ns=kafka
ports=(19092 19093 19094 8081 9090 3000)

free_ports() {
  local p pids
  for p in "${ports[@]}"; do
    pids=$(lsof -ti tcp:"$p" -sTCP:LISTEN 2>/dev/null || true)
    if [[ -n "$pids" ]]; then
      kill $pids 2>/dev/null || true
    fi
  done
}

free_ports
sleep 0.2

trap 'kill $(jobs -p) 2>/dev/null' EXIT
kubectl -n "$ns" port-forward pod/ingest-platform-dual-role-0 19092:9094 &
kubectl -n "$ns" port-forward pod/ingest-platform-dual-role-1 19093:9094 &
kubectl -n "$ns" port-forward pod/ingest-platform-dual-role-2 19094:9094 &
kubectl -n "$ns" port-forward svc/karapace 8081:8081 &
kubectl -n "$ns" port-forward svc/prometheus 9090:9090 &
kubectl -n "$ns" port-forward svc/grafana 3000:3000 &
echo "brokers 19092-19094  karapace 8081  prometheus 9090  grafana 3000  Ctrl-C stops all"
wait
