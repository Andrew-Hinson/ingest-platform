# Ingest Platform

An application attaches to a Cluster by declaring a Project. This product applies that declaration.

## Language

**Cluster**:
A named Kubernetes runtime that already hosts Kafka and Connect.

**Project**:
The identity of a consumer git repo attached to a Cluster.

**Prefix**:
A Project override for resource names, defaulting to the Project name under the Cluster template `{prefix}.{name}`.

**Platform stack**:
The shared Kafka and Connect worker on a Cluster.

**Apply**:
ingestctl's reconciliation of a Project onto a Cluster.
