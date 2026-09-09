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
  description = "Kafka topic to create. Caller supplies this (tenant YAML later)."
  default     = "acme.orders"
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
  description = "Tenant principal / KafkaUser name."
  default     = "acme"
}

variable "prefix" {
  type        = string
  description = "ACL prefix. Tenant can RW this prefix, not the cluster."
  default     = "acme."
}
