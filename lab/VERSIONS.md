Strimzi Cluster Operator: 1.2.0
  install: https://github.com/strimzi/strimzi-kafka-operator/releases/download/1.2.0/strimzi-cluster-operator-1.2.0.yaml
Kafka: 4.3.1
metadataVersion: 4.3-IV0
CRD API: kafka.strimzi.io/v1
Karapace: 5.0.3
Postgres: 17.11
KafkaConnect: 4.3.1
Debezium PostgreSQL connector: 3.6.2.Final
Avro converter: io.confluent.connect.avro.AvroConverter 8.3.1
  registry: Karapace http://karapace.kafka.svc.cluster.local:8081
Go producer: github.com/twmb/franz-go v1.21.6
Go consumer: github.com/twmb/franz-go v1.21.6
Terraform: 1.16.1
Terraform Kubernetes provider: hashicorp/kubernetes 3.2.1
  resource: kubernetes_manifest
ingestctl: gopkg.in/yaml.v3 v3.0.1
CI: GitHub Actions
  Go: 1.27.0
