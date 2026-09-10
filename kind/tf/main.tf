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
  prefix    = var.prefix
  namespace = var.namespace
  cluster   = var.cluster
  topic_ops = var.topic_ops
}
