locals {
  iceberg_tables = join(",", [for t in var.tables : "${var.glue_database}.${t.iceberg_table}"])
}

resource "aws_mskconnect_connector" "this" {
  name                 = var.name
  kafkaconnect_version = var.kafkaconnect_version

  capacity {
    provisioned_capacity {
      mcu_count    = 1
      worker_count = 1
    }
  }

  connector_configuration = {
    "connector.class"                    = "org.apache.iceberg.connect.IcebergSinkConnector"
    "tasks.max"                          = "1"
    "topics"                             = join(",", var.topics)
    "iceberg.tables"                     = local.iceberg_tables
    "iceberg.tables.auto-create-enabled" = "true"
    "iceberg.catalog"                    = "glue"
    "iceberg.catalog.catalog-impl"       = "org.apache.iceberg.aws.glue.GlueCatalog"
    "iceberg.catalog.io-impl"            = "org.apache.iceberg.aws.s3.S3FileIO"
    "iceberg.catalog.warehouse"          = "s3://${var.warehouse_bucket}/"
    "iceberg.catalog.glue.skip-archive"  = "true"
    "key.converter"                      = "org.apache.kafka.connect.json.JsonConverter"
    "value.converter"                    = "org.apache.kafka.connect.json.JsonConverter"
    "key.converter.schemas.enable"       = "true"
    "value.converter.schemas.enable"     = "true"
  }

  kafka_cluster {
    apache_kafka_cluster {
      bootstrap_servers = var.bootstrap_servers

      vpc {
        security_groups = var.security_groups
        subnets         = var.subnet_ids
      }
    }
  }

  kafka_cluster_client_authentication {
    authentication_type = "IAM"
  }

  kafka_cluster_encryption_in_transit {
    encryption_type = "TLS"
  }

  plugin {
    custom_plugin {
      arn      = var.plugin_arn
      revision = var.plugin_revision
    }
  }

  service_execution_role_arn = var.role_arn
}
