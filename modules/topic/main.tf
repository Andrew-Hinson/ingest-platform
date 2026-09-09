resource "kubernetes_manifest" "topic" {
  manifest = {
    apiVersion = "kafka.strimzi.io/v1"
    kind       = "KafkaTopic"
    metadata = {
      name      = var.name
      namespace = var.namespace
      labels = {
        "strimzi.io/cluster" = var.cluster
      }
    }
    spec = {
      partitions = var.partitions
      replicas   = var.replicas
      config = {
        "min.insync.replicas" = tostring(var.min_insync_replicas)
      }
    }
  }

  computed_fields = [
    "metadata.annotations",
    "metadata.labels",
    "status",
  ]
}
