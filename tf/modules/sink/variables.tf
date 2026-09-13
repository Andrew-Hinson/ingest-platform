variable "name" {
  type = string
}

variable "plugin_arn" {
  type = string
}

variable "plugin_revision" {
  type    = number
  default = 1
}

variable "role_arn" {
  type = string
}

variable "bootstrap_servers" {
  type = string
}

variable "subnet_ids" {
  type = list(string)
}

variable "security_groups" {
  type = list(string)
}

variable "topics" {
  type = list(string)
}

variable "warehouse_bucket" {
  type = string
}

variable "glue_database" {
  type = string
}

variable "tables" {
  type = list(object({
    topic_name    = string
    iceberg_table = string
  }))
}

variable "kafkaconnect_version" {
  type    = string
  default = "2.7.1"
}
