# ingest-platform

Self-service ingestion: YAML + Terraform give teams Kafka topics, ACLs, and a Debezium connector.

**Now:** Kind + Strimzi Kafka 4.3.1, topic `lab.events`, Karapace, Prometheus, Grafana. Go producer/consumer against local brokers.

Pinned versions: `lab/VERSIONS.md`.

## Lab standup

Needs `kind` and `kubectl`. Run in order. Later steps wait on earlier ones.

Create the Kind cluster named `ingest-platform`.

```bash
kind create cluster --name ingest-platform --config lab/kind/config.yaml
```

Create the `kafka` namespace. Needed before operator, cluster, and sidecars.

```bash
kubectl create namespace kafka
```

Install the Strimzi Cluster Operator 1.2.0 into `kafka` (rewrites the default namespace in the release YAML).

```bash
curl -fsSL https://github.com/strimzi/strimzi-kafka-operator/releases/download/1.2.0/strimzi-cluster-operator-1.2.0.yaml \
  | sed 's/namespace: myproject/namespace: kafka/g' \
  | kubectl apply -f - -n kafka
```

Wait until the operator Deployment is ready. Kafka CRs fail if this is still rolling.

```bash
kubectl -n kafka rollout status deployment/strimzi-cluster-operator --timeout=180s
```

Apply the Kafka cluster (3 dual-role brokers) and topic `lab.events`.

```bash
kubectl apply -f lab/strimzi/config.yaml -n kafka
```

Wait until the Kafka CR reports Ready. Brokers are not usable before this.

```bash
kubectl -n kafka wait kafka/ingest-platform --for=condition=Ready --timeout=600s
```

Confirm the topic exists.

```bash
kubectl -n kafka get kafkatopic lab.events
```

Apply Karapace (schema registry). Needs a Ready bootstrap.

```bash
kubectl apply -f lab/karapace -n kafka
```

Apply Prometheus and Grafana.

```bash
kubectl apply -f lab/grafana -n kafka
```

## Access

`./lab/port-forward.sh` maps brokers `19092-19094`, Karapace `8081`, Prometheus `9090`, Grafana `3000`. Ctrl-C stops all.

Broker bootstrap for clients: `127.0.0.1:19092` (`KAFKA_BOOTSTRAP` overrides).

## Clients

```bash
go run -C clients/producer .
```

```bash
go run -C clients/consumer . -id a -group lab-events -commit auto
```

In-cluster produce/debug: `kubectlcmds.md`.
