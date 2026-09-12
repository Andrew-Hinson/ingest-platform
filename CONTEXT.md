# Ingest Platform

A CLI and modules. A Project declares Tables. Developers write rows. Apply creates the Database objects and pipes each Table to Kafka and the Warehouse. Project YAML lives in the consumer git repo.

## Language

**Cluster**:
A named EKS runtime that already hosts Kafka, Connect, and the Warehouse.
_Avoid_: kind, local cluster

**Project**:
The identity of a consumer git repo attached to a Cluster.

**Prefix**:
A Project override for resource names, defaulting to the Project name under the Cluster template `{prefix}.{name}`.

**Platform stack**:
The shared Kafka, Connect worker, and Warehouse on a Cluster.

**Warehouse**:
The shared S3 bucket and Iceberg catalog for a Cluster. Projects do not create one. Prefix separates Iceberg tables.
_Avoid_: landing zone, data lake, per-Project bucket

**Instance**:
The Postgres server a Project's Databases live on. It is RDS. Exactly one Project creates a given Instance; other Projects name it. Creator name defaults to the Project name. There is no Cluster Postgres.
_Avoid_: cluster, RDS as the generic name

**Database**:
A named Postgres database. A `databases:` list in YAML means this Project creates those Databases. A Table always names its Database. If this Project creates an Instance and omits `databases:`, Apply creates a Database named after the Project.
_Avoid_: schema

**Schema**:
A Postgres namespace inside a Database. Default `public`. A Table may override it.
_Avoid_: using this word for DDL or for Database

**Table**:
A named relation this Project owns. Always declared, with columns as DDL, and a primary key. Name is the relation (`orders`), not `public.orders`. A Table is the source of a Kafka topic and an Iceberg table.
_Avoid_: entity, model, schema

**Iceberg table**:
The derived lake relation for a Table. It lives in the Warehouse, namespaced by Prefix. Iceberg is a format, not a Cluster service.
_Avoid_: landing zone, lakehouse

**Instance Secret**:
The Secrets Manager store named after the Instance. Org/secops creates it. Apply and workloads retrieve it. Not in Project YAML.
_Avoid_: password in YAML, generated credentials, Secrets created by Apply, Kubernetes Secret

**Apply**:
ingestctl's reconciliation of a Project: Instance, Database, Table, Kafka path, and Iceberg table. Laptop or CI. Prints endpoint, Database, user, and Secret name. Does not print the password. Does not create Secrets.
