resource "kubernetes_manifest" "user" {
  manifest = {
    apiVersion = "kafka.strimzi.io/v1"
    kind       = "KafkaUser"
    metadata = {
      name      = var.principal
      namespace = var.namespace
      labels = {
        "strimzi.io/cluster" = var.cluster
      }
    }
    spec = {
      authorization = {
        type = "simple"
        acls = [
          {
            resource = {
              type        = "topic"
              name        = var.prefix
              patternType = "prefix"
            }
            operations = ["Read", "Write", "Describe"]
          },
          {
            resource = {
              type        = "group"
              name        = var.prefix
              patternType = "prefix"
            }
            operations = ["Read"]
          },
        ]
      }
    }
  }

  computed_fields = [
    "metadata.annotations",
    "metadata.labels",
    "status",
  ]
}
