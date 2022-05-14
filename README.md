# Capsule Addon for CloudCasa.io by Catalog

Empower your self-service and effortless Kubernetes backups and restores taking full advantage of the multi-tenancy!

## Prerequisites

- A valid API token for [CloudCasa](https://cloudcasa.io)
- [Capsule Operator](https://capsule.clastix.io) installed in your cluster

## Installation

Create a Secret named `cloudcasa-api-token` in the `capsule-system` Namespace as follows.

```
kubectl --namespace capsule-system \
    create secret generic cloudcasa-api-token \
    --from-literal=token=<REDACTED>
```

Deploy the addon as follows.

```
kubectl apply -f https://raw.githubusercontent.com/clastix/capsule-addon-cloudcasa/master/config/installer.yaml
```
