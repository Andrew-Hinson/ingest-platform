output "host" {
  value      = var.name
  depends_on = [kubernetes_stateful_set_v1.instance]
}
