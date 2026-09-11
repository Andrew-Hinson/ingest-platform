# ingest-platform

Self-service ingestion: YAML + Terraform give teams an Instance, Kafka topics, ACLs, and a Debezium connector.

**Now:** Kind Cluster + Strimzi Kafka 4.3.1, topic `lab.events`, Karapace, KafkaConnect with Debezium, Prometheus, Grafana. Apply creates the Instance from Project YAML. Go producer/consumer against local brokers.

Pinned versions: `kind/VERSIONS.md`. PRs run `.github/workflows/ci.yml` (gofmt, vet, test, terraform fmt/validate).

## Platform standup

Needs `kind` and `kubectl`. Run in order. Later steps wait on earlier ones.

Create the Kind Cluster named `ingest-platform`.

```bash
kind create cluster --name ingest-platform --config kind/kind/config.yaml
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
kubectl apply -f kind/strimzi/config.yaml -n kafka
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
kubectl apply -f kind/karapace -n kafka
```

Apply KafkaConnect (Debezium plugin, Avro converter → Karapace). First apply builds a Connect image; Ready can take several minutes. No connector CR until Apply. No Cluster Postgres; Apply creates the Instance.

```bash
kubectl apply -f kind/connect -n kafka
```

Wait until the KafkaConnect CR reports Ready.

```bash
kubectl -n kafka wait kafkaconnect/ingest-platform --for=condition=Ready --timeout=600s
```

Apply Prometheus and Grafana.

```bash
kubectl apply -f kind/grafana -n kafka
```

## Apply Instance + topic + ACL + connector

`ingestctl` reads Project YAML, writes per-Project local Terraform state under `.ingestctl/`, runs `terraform init/apply` against `kind/tf`. Creator Apply brings up the Instance (in-cluster Postgres). Credentials are retrieved from a Secret named after the Instance. Apply does not create that Secret. YAML has no secret fields. Modules do not manage `lab.events`.

The Secret must already exist (org/secops). Name is the Instance name (`acme`). Keys: `user`, `password`. Password length and randomization are not set by this product. Apply fails if the Secret is missing.

```bash
go run -C cmd/ingestctl . apply -f examples/acme.yaml
```

Apply prints `endpoint`, `database`, `user`, and `secret`. It does not print the password. The Postgres pod and Connect retrieve `user`/`password` at runtime. Terraform never stores them.

```bash
kubectl -n kafka get secret acme
```

```bash
kubectl -n kafka get statefulset acme
```

```bash
kubectl -n kafka get kafkatopic acme.public.orders
```

```bash
kubectl -n kafka get kafkauser acme
```

```bash
kubectl -n kafka get kafkaconnector acme-orders-cdc
```

## Access

`./kind/port-forward.sh` maps brokers `19092-19094`, Karapace `8081`, Prometheus `9090`, Grafana `3000`. Ctrl-C stops all.

Broker bootstrap for clients: `127.0.0.1:19092` (`KAFKA_BOOTSTRAP` overrides).

## Clients

```bash
go run -C clients/producer .
```

```bash
go run -C clients/consumer . -id a -group lab-events -commit auto
```

In-cluster produce/debug: `kubectlcmds.md`.
