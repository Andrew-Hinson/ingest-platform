# Shared Cluster

A Config names an existing Cluster and never creates MSK, Connect, or the Warehouse. Those are shared per environment so a second Config adds topics and Iceberg tables, not a second Kafka bill. Dedicated stacks were rejected as unusual on AWS and outside a PaaS.
