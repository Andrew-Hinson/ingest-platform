# YAML is the only DDL path

Table columns in the Config are the DDL. Apply creates missing Tables and fails when live columns do not match YAML. The app writes rows only. App-owned migrations were rejected because the connector include-list and Iceberg schema cannot be guessed from a drifting database.
