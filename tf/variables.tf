variable "namespace" {
  type    = string
  default = "kafka"
}

variable "instance_name" {
  type        = string
  description = "Instance name. Also the Secret name to retrieve."
  default     = "acme"
}

variable "instance_create" {
  type        = bool
  description = "Creator Apply brings up the Instance."
  default     = false
}

variable "instance_database" {
  type        = string
  description = "Database name from Project YAML."
  default     = "acme"
}

variable "instance_creator" {
  type        = string
  description = "Project that creates the Instance."
  default     = "acme"
}

variable "cluster" {
  type    = string
  default = "ingest-platform"
}

variable "tables" {
  type = list(object({
    topic_name             = string
    partitions             = number
    connector_name         = string
    connector_class        = string
    connector_database     = string
    connector_table        = string
    connector_topic_prefix = string
    tasks_max              = number
    publication_name       = string
  }))
  description = "Derived topic and connector per Table."
  default = [{
    topic_name             = "acme.public.orders"
    partitions             = 3
    connector_name         = "acme-orders-cdc"
    connector_class        = "io.debezium.connector.postgresql.PostgresConnector"
    connector_database     = "acme"
    connector_table        = "public.orders"
    connector_topic_prefix = "acme"
    tasks_max              = 1
    publication_name       = "acme_orders_cdc"
  }]
}

variable "replicas" {
  type    = number
  default = 3
}

variable "min_insync_replicas" {
  type    = number
  default = 2
}

variable "principal" {
  type        = string
  description = "KafkaUser name."
  default     = "acme"
}

variable "acl_resource" {
  type        = string
  description = "ACL resource pattern from YAML resource. Project can RW this pattern, not the cluster."
  default     = "acme."
}

variable "topic_ops" {
  type        = list(string)
  description = "Topic ACL operations."
  default     = ["Read", "Write", "Describe"]
}

variable "connector_hostname" {
  type        = string
  description = "Postgres hostname Connect can reach."
  default     = "postgres"
}

variable "connector_port" {
  type        = number
  description = "Postgres port."
  default     = 5432
}
