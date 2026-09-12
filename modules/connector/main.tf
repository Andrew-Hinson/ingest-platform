resource "kubernetes_manifest" "connector" {
  manifest = {
    apiVersion = "kafka.strimzi.io/v1"
    kind       = "KafkaConnector"
    metadata = {
      name      = var.name
      namespace = var.namespace
      labels = {
        "strimzi.io/cluster" = var.cluster
      }
    }
    spec = {
      class    = var.class
      tasksMax = var.tasks_max
      config = {
        "database.hostname"           = var.database_hostname
        "database.port"               = tostring(var.database_port)
        "database.user"               = format("$${secrets:%s/%s:user}", var.namespace, var.secret_name)
        "database.password"           = format("$${secrets:%s/%s:password}", var.namespace, var.secret_name)
        "database.dbname"             = var.database
        "topic.prefix"                = var.topic_prefix
        "table.include.list"          = var.table
        "plugin.name"                 = "pgoutput"
        "slot.name"                   = replace(var.name, "-", "_")
        "publication.name"            = var.publication_name
        "publication.autocreate.mode" = "filtered"
      }
    }
  }

  computed_fields = [
    "metadata.annotations",
    "metadata.labels",
    "status",
  ]
}
