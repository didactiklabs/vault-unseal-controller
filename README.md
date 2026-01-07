# Vault Unseal Controller

[![CI](https://github.com/didactiklabs/vault-unseal-controller/actions/workflows/test.yml/badge.svg)](https://github.com/didactiklabs/vault-unseal-controller/actions/workflows/test.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/didactiklabs/vault-unseal-controller)](https://goreportcard.com/report/github.com/didactiklabs/vault-unseal-controller)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

A Kubernetes operator that automatically unseals HashiCorp Vault instances when they become sealed.

## Overview

The Vault Unseal Controller monitors Vault nodes and automatically triggers unseal jobs when sealed nodes are detected. This ensures high availability of your Vault infrastructure without manual intervention.

### Key Features

- 🔄 **Automatic unsealing** of sealed Vault nodes
- 🌐 **Multi-node support** for Vault clusters
- 🔒 **Secure secret management** using Kubernetes secrets
- 🔐 **TLS support** with custom CA certificates or skip verification
- ⚙️ **Configurable retry logic** for failed unseal attempts

## Architecture

The controller creates Kubernetes Jobs in the same namespace as your unseal keys secrets, allowing proper secret mounting across namespaces while maintaining security boundaries.

## Prerequisites

- Kubernetes cluster v1.31+
- kubectl v1.31+
- Go 1.25+ (for development)
- Docker or Podman (for building images)

## Installation

### Using Pre-built Bundle

Download and apply the latest release bundle:

```bash
kubectl apply -f https://github.com/didactiklabs/vault-unseal-controller/releases/latest/download/bundle.yaml
```

### Building from Source

```bash
# Build and push images
make docker-build docker-push IMG=<your-registry>/vault-unseal-controller VERSION=<version>
make docker-build-unsealer docker-push-unsealer UNSEALER_IMG=<your-registry>/vault-unsealer VERSION=<version>

# Deploy to cluster
make deploy IMG=<your-registry>/vault-unseal-controller VERSION=<version>
```

## Usage

### Creating an Unseal Resource

Create a secret with your Vault unseal keys:

```bash
kubectl create secret generic vault-keys \
  --from-literal=key1=<key-1> \
  --from-literal=key2=<key-2> \
  --from-literal=key3=<key-3> \
  -n vault-namespace
```

Create an Unseal custom resource:

```yaml
apiVersion: platform.didactiklabs.io/v1alpha1
kind: Unseal
metadata:
  name: vault-cluster
spec:
  vaultNodes:
    - https://my-vault.example.com:8200 # Exposed Vault URL
  unsealKeysSecretRef:
    name: vault-keys
    namespace: vault-namespace
```

Or you can iterate over a list of available nodes endpoints:
```yaml
apiVersion: platform.didactiklabs.io/v1alpha1
kind: Unseal
metadata:
  name: vault-cluster
spec:
  vaultNodes:
    - https://vault-0.example.com:8200
    - https://vault-1.example.com:8200
    - https://vault-2.example.com:8200
  unsealKeysSecretRef:
    name: vault-keys
    namespace: vault-namespace
```

Apply it:

```bash
kubectl apply -f unseal.yaml
```

### With TLS Skip Verify

For development or self-signed certificates:

```yaml
apiVersion: platform.didactiklabs.io/v1alpha1
kind: Unseal
metadata:
  name: vault-cluster-dev
spec:
  vaultNodes:
    - https://vault.example.com:8200
  unsealKeysSecretRef:
    name: vault-keys
    namespace: default
  tlsSkipVerify: true
```

### With Custom CA Certificate

For production with custom CA:

```bash
kubectl create secret generic vault-ca-cert \
  --from-file=ca.crt=path/to/ca.crt \
  -n vault-namespace # CA certificate secret must be in the same namespace as the unseal keys
```

```yaml
apiVersion: platform.didactiklabs.io/v1alpha1
kind: Unseal
metadata:
  name: vault-cluster-prod
spec:
  vaultNodes:
    - https://vault-0.vault.svc:8200
  unsealKeysSecretRef:
    name: vault-keys
    namespace: vault-namespace
  caCertSecret: vault-ca-cert
  retryCount: 5
```

## Configuration

### Unseal Spec

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `vaultNodes` | `[]string` | Yes | List of Vault node URLs to monitor |
| `unsealKeysSecretRef` | `SecretRef` | Yes | Reference to secret containing unseal keys |
| `caCertSecret` | `string` | No | Name of secret containing CA certificate (must be in same namespace as unseal keys) |
| `tlsSkipVerify` | `bool` | No | Skip TLS certificate verification (default: false) |
| `retryCount` | `int32` | No | Number of retry attempts for unseal jobs (default: 3) |

### Status Fields

The controller maintains status information:

- `vaultStatus`: Current state (UNSEALED, UNSEALING, CLEANING)
- `sealedNodes`: List of currently sealed Vault nodes

## Development

### Running Tests

```bash
# Unit tests
make test

# E2E tests (requires Kind)
make test-e2e

# Lint
make lint
```

### Building Locally

```bash
# Build controller binary
make build

# Build unsealer binary
make build-unsealer

# Run locally (outside cluster)
make run
```

### Releasing

Create a new git tag to trigger the release workflow:

```bash
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0
```

This will:
- Build and publish multi-arch Docker images (amd64, arm64)
- Create a GitHub release with GoReleaser
- Generate and attach deployment bundle

## How It Works

1. **Monitoring**: The controller periodically checks the seal status of configured Vault nodes
2. **Detection**: When sealed nodes are detected, status transitions to `UNSEALING`
3. **Job Creation**: A Kubernetes Job is created for each sealed node in the secret's namespace
4. **Unsealing**: The job uses unseal keys from the secret to unseal the node
5. **Cleanup**: After successful unsealing, jobs are cleaned up and status returns to `UNSEALED`

## Troubleshooting

### Pods Can't Pull Images

Ensure the controller deployment has `imagePullPolicy: IfNotPresent` or that images are available in your registry.

### Jobs Fail to Mount Secrets

Verify that:
- The secret exists in the specified namespace
- Jobs are created in the same namespace as the secret
- ServiceAccount has proper RBAC permissions

### Unseal Jobs Keep Failing

Check:
- Unseal keys are correct and base64-encoded
- Vault nodes are reachable from the cluster
- TLS configuration matches your Vault setup
- RetryCount is sufficient for your environment

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Make your changes with tests
4. Ensure all tests pass: `make test lint`
5. Submit a pull request

## License

Copyright 2026 Didactik Labs.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
