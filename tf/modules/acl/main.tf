resource "kafka_acl" "this" {
  for_each = toset(var.ops)

  resource_name                = var.prefix
  resource_type                = "Topic"
  resource_pattern_type_filter = "Prefixed"
  acl_principal                = "User:${var.principal}"
  acl_host                     = "*"
  acl_operation                = each.value
  acl_permission_type          = "Allow"
}
