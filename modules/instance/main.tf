resource "kubernetes_service_v1" "instance" {
  metadata {
    name      = var.name
    namespace = var.namespace
    labels = {
      ingest-platform-instance = var.name
      ingest-platform-creator  = var.creator
    }
  }

  spec {
    selector = {
      ingest-platform-instance = var.name
    }

    port {
      name        = "postgres"
      port        = 5432
      target_port = 5432
    }
  }
}

resource "kubernetes_stateful_set_v1" "instance" {
  wait_for_rollout = true

  timeouts {
    create = "3m"
    update = "3m"
  }

  metadata {
    name      = var.name
    namespace = var.namespace
    labels = {
      ingest-platform-instance = var.name
      ingest-platform-creator  = var.creator
    }
  }

  spec {
    service_name = var.name
    replicas     = 1

    selector {
      match_labels = {
        ingest-platform-instance = var.name
      }
    }

    template {
      metadata {
        labels = {
          ingest-platform-instance = var.name
          ingest-platform-creator  = var.creator
        }
      }

      spec {
        container {
          name  = "postgres"
          image = "postgres:17.11"

          args = [
            "postgres",
            "-c",
            "wal_level=logical",
            "-c",
            "max_wal_senders=4",
            "-c",
            "max_replication_slots=4",
          ]

          port {
            container_port = 5432
          }

          env {
            name = "POSTGRES_USER"
            value_from {
              secret_key_ref {
                name = var.name
                key  = "user"
              }
            }
          }

          env {
            name = "POSTGRES_PASSWORD"
            value_from {
              secret_key_ref {
                name = var.name
                key  = "password"
              }
            }
          }

          env {
            name  = "POSTGRES_DB"
            value = var.database
          }

          env {
            name  = "PGDATA"
            value = "/var/lib/postgresql/data/pgdata"
          }

          resources {
            requests = {
              cpu    = "100m"
              memory = "256Mi"
            }
            limits = {
              cpu    = "500m"
              memory = "512Mi"
            }
          }

          readiness_probe {
            exec {
              command = ["sh", "-c", "pg_isready -U \"$POSTGRES_USER\" -d \"$POSTGRES_DB\""]
            }
            period_seconds = 5
          }

          volume_mount {
            name       = "data"
            mount_path = "/var/lib/postgresql/data"
          }
        }
      }
    }

    volume_claim_template {
      metadata {
        name = "data"
      }

      spec {
        access_modes = ["ReadWriteOnce"]

        resources {
          requests = {
            storage = "1Gi"
          }
        }
      }
    }
  }
}
