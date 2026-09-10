variable "namespace" {
  type    = string
  default = "kafka"
}

variable "cluster" {
  type    = string
  default = "ingest-platform"
}

variable "topic_name" {
  type        = string
  description = "Kafka topic to create."
  default     = "acme.public.orders"
}

variable "partitions" {
  type    = number
  default = 3
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

variable "connector_name" {
  type        = string
  description = "KafkaConnector metadata.name."
  default     = "acme-orders-cdc"
}

variable "connector_class" {
  type        = string
  description = "Connect connector class."
  default     = "io.debezium.connector.postgresql.PostgresConnector"
}

variable "connector_database" {
  type        = string
  description = "Postgres database name. YAML database, not the Platform db."
  default     = "acme"
}

variable "connector_table" {
  type        = string
  description = "table.include.list."
  default     = "public.orders"
}

variable "connector_topic_prefix" {
  type        = string
  description = "Debezium topic.prefix."
  default     = "acme"
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

variable "connector_user" {
  type        = string
  description = "Postgres user."
  default     = "lab"
}

variable "connector_password" {
  type        = string
  description = "Postgres password."
  default     = "lab"
  sensitive   = true
}
