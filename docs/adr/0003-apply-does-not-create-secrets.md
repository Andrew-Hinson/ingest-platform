# Apply does not create Secrets

The Instance Secret is created by org/secops and named after the Instance. Apply retrieves user and password, fails if the secret is missing, and never prints the password. Config YAML has no secret fields. Apply-created passwords were rejected so credential ownership stays outside the CLI.
