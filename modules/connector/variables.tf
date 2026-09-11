variable "name" {
  type        = string
  description = "KafkaConnector metadata.name."
}

variable "class" {
  type        = string
  description = "Connect connector class."
}

variable "database" {
  type        = string
  description = "Postgres database name (database.dbname)."
}

variable "table" {
  type        = string
  description = "table.include.list, e.g. public.orders."
}

variable "topic_prefix" {
  type        = string
  description = "Debezium topic.prefix."
}

variable "namespace" {
  type        = string
  description = "Namespace of the KafkaConnect cluster."
}

variable "cluster" {
  type        = string
  description = "strimzi.io/cluster label. Must match the KafkaConnect CR name."
}

variable "database_hostname" {
  type        = string
  description = "Postgres hostname Connect can reach."
}

variable "database_port" {
  type        = number
  description = "Postgres port."
}

variable "secret_name" {
  type        = string
  description = "Instance Secret Connect retrieves at runtime. Keys user and password."
}
