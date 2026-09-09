variable "name" {
  type        = string
  description = "Kafka topic name. Also the KafkaTopic metadata.name."
}

variable "namespace" {
  type        = string
  description = "Namespace of the Strimzi cluster."
}

variable "cluster" {
  type        = string
  description = "strimzi.io/cluster label. Must match the Kafka CR name."
}

variable "partitions" {
  type        = number
  description = "Partition count."
}

variable "replicas" {
  type        = number
  description = "Replication factor."
}

variable "min_insync_replicas" {
  type        = number
  description = "min.insync.replicas topic config."
}
