module "topic" {
  for_each = { for t in var.tables : t.topic_name => t }

  source = "../modules/topic"

  name                = each.value.topic_name
  namespace           = var.namespace
  cluster             = var.cluster
  partitions          = each.value.partitions
  replicas            = var.replicas
  min_insync_replicas = var.min_insync_replicas
}

module "acl" {
  source = "../modules/acl"

  principal = var.principal
  prefix    = var.acl_resource
  namespace = var.namespace
  cluster   = var.cluster
  topic_ops = var.topic_ops
}

module "connector" {
  for_each = { for t in var.tables : t.connector_name => t }

  source = "../modules/connector"

  name              = each.value.connector_name
  class             = each.value.connector_class
  database          = each.value.connector_database
  table             = each.value.connector_table
  topic_prefix      = each.value.connector_topic_prefix
  tasks_max         = each.value.tasks_max
  publication_name  = each.value.publication_name
  namespace         = var.namespace
  cluster           = var.cluster
  database_hostname = var.connector_hostname
  database_port     = var.connector_port
  secret_name       = var.instance_name
}
