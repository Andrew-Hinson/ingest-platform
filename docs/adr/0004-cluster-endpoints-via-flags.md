# Cluster endpoints via flags

Apply resolves MSK, Connect, Warehouse, and RDS network placement from flags and `RELAY_*` env vars, not from git. The Cluster already exists and its endpoints are environment-specific. A clusters file in the repo was rejected so secrets and account topology do not land in the Config's source tree.
