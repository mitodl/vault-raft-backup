## vault-raft-backup

Vault Raft backup is a lean tool for creating snapshots of the Raft integrated storage in Hashicorp Vault, and transferring those backups to AWS S3.

### Prerequsities

If executing as an ad-hoc compile and run (i.e. `go run`), then the dependencies and requirements can be viewed in the [go.mod](go.mod) file. Additional setup requirements are as follows:

- Connectivity to a functioning Vault server cluster with Raft integrated storage.
- Authentication and authorization against the Vault server cluster for executing Raft snapshots.
- A local filesystem with permissions and storage capable of staging the snapshot.
- Authentication and authorization against an AWS account for listing, reading, and writing objects to a S3 bucket.
- A S3 bucket capable of storing the snapshot.

The Vault policy for executing Raft snapshots appears like:

```hcl
path "sys/storage/raft/snapshot" {
  capabilities = ["read"]
}
```

### Usage

The following environment variables are read for configuration of the backup tool. This usage is due to the expectation that this tool will be executed as part of automation e.g. pipeline, service, orchestator, etc., and also because some inputs are sensitive.

```
# equivalent to VAULT_ADDR with vault cli executable
export VAULT_ADDR=<vault server cluster address>
# equivalent to VAULT_TOKEN with vault cli executable; if using aws iam auth, then set this equal to "aws-iam" instead
export VAULT_TOKEN=<vault authentication token>
# equivalent to VAULT_SKIP_VERIFY with vault cli executable
export VAULT_SKIP_VERIFY=<boolean>
export VAULT_SNAPSHOT_PATH=<path to local filesystem for snapshot staging>
export S3_BUCKET=<name of s3 bucket for snapshot transfer and storage>
# this is prepended to the base filename in VAULT_SNAPSHOT_PATH
export S3_PREFIX=<snapshot filename prefix during s3 transfer>
export AWS_REGION=<aws region for client session>
```

Additionally, AWS authentication must be provided with environment variables or other standard methods. The AWS Golang SDK will automatically read authentication information as per normal (i.e. IAM instance profile, `AWS_PROFILE`, etc.).
