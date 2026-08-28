# ingest-platform

Self-service ingestion platform: teams get Kafka topics, ACLs, and a Debezium connector from YAML and Terraform. Flink writes Iceberg. Operated on Kind/Strimzi with scripted failure drills.

## WIP Commands

# 1. Kind
```
kind create cluster --name ingest-platform --config lab/kind/config.yaml
kubectl cluster-info --context kind-ingest-platform
```
# 2. namespace / operator
```
kubectl create namespace kafka
curl -fsSL https://github.com/strimzi/strimzi-kafka-operator/releases/download/1.2.0/strimzi-cluster-operator-1.2.0.yaml \
  | sed 's/namespace: myproject/namespace: kafka/g' \
  | kubectl apply -f - -n kafka
kubectl -n kafka rollout status deployment/strimzi-cluster-operator --timeout=180s
```
# 3. Kafka / lab.events
```
kubectl apply -f lab/strimzi/config.yaml -n kafka
```
# 4. wait / check
```
kubectl -n kafka wait kafka/ingest-platform --for=condition=Ready --timeout=600s
kubectl -n kafka get kafkatopic lab.events```
```

