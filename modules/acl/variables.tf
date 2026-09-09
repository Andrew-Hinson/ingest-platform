variable "principal" {
  type        = string
  description = "Tenant principal. KafkaUser metadata.name."
}

variable "prefix" {
  type        = string
  description = "Topic and group name prefix this principal can RW."
}

variable "namespace" {
  type        = string
  description = "Namespace of the Strimzi cluster."
}

variable "cluster" {
  type        = string
  description = "strimzi.io/cluster label. Must match the Kafka CR name."
}
