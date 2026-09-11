variable "name" {
  type        = string
  description = "Instance name. Also the Secret name to retrieve. Service and StatefulSet metadata.name."
}

variable "namespace" {
  type        = string
  description = "Namespace of the Kind Cluster."
}

variable "database" {
  type        = string
  description = "POSTGRES_DB created with the Instance."
}

variable "creator" {
  type        = string
  description = "Project that creates this Instance."
}
