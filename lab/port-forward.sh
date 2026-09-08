#!/usr/bin/env bash
set -euo pipefail
ns=kafka
trap 'kill $(jobs -p) 2>/dev/null' EXIT
kubectl -n "$ns" port-forward pod/ingest-platform-dual-role-0 19092:9094 &
kubectl -n "$ns" port-forward pod/ingest-platform-dual-role-1 19093:9094 &
kubectl -n "$ns" port-forward pod/ingest-platform-dual-role-2 19094:9094 &
kubectl -n "$ns" port-forward svc/karapace 8081:8081 &
echo "brokers 19092-19094  karapace 8081  Ctrl-C stops all"
wait