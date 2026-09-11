module "instance" {
  count  = var.instance_create ? 1 : 0
  source = "../../modules/instance"

  name      = var.instance_name
  namespace = var.namespace
  database  = var.instance_database
  creator   = var.instance_creator
}

module "topic" {
  source = "../../modules/topic"

  name                = var.topic_name
  namespace           = var.namespace
  cluster             = var.cluster
  partitions          = var.partitions
  replicas            = var.replicas
  min_insync_replicas = var.min_insync_replicas
}

module "acl" {
  source = "../../modules/acl"

  principal = var.principal
  prefix    = var.acl_resource
  namespace = var.namespace
  cluster   = var.cluster
  topic_ops = var.topic_ops
}

module "connector" {
  source     = "../../modules/connector"
  depends_on = [module.instance]

  name              = var.connector_name
  class             = var.connector_class
  database          = var.connector_database
  table             = var.connector_table
  topic_prefix      = var.connector_topic_prefix
  namespace         = var.namespace
  cluster           = var.cluster
  database_hostname = var.connector_hostname
  database_port     = var.connector_port
  secret_name       = var.instance_name
}
