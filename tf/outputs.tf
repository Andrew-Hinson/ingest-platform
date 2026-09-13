output "rds_endpoint" {
  value = try(module.rds[0].endpoint, "")
}
