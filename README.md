# ingest-platform

Self-service ingestion: YAML + Terraform give teams an Instance, Kafka topics, ACLs, and a Debezium connector.

A Project names an existing Cluster. Apply does not create the Cluster. PRs run `.github/workflows/ci.yml` (gofmt, vet, test, terraform fmt/validate).

## Apply

`ingestctl` reads Project YAML, writes per-Project Terraform state under `.ingestctl/`, and runs `terraform init/apply` against `tf/`. Credentials live in Secrets Manager named after the Instance. Apply does not create that Secret. YAML has no secret fields.

```bash
go run -C cmd/ingestctl . apply -f examples/acme.yaml
```

Apply prints `endpoint`, `database`, `user`, and `secret`. It does not print the password. Terraform never stores the password.
