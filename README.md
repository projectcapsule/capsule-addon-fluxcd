[![Latest release](https://img.shields.io/github/v/release/maxgio92/capsule-addon-fluxcd?style=for-the-badge)](https://github.com/maxgio92/capsule-addon-fluxcd/releases/latest)
[![License](https://img.shields.io/github/license/maxgio92/capsule-addon-fluxcd?style=for-the-badge)](COPYING)
![Go version](https://img.shields.io/github/go-mod/go-version/maxgio92/capsule-addon-fluxcd?style=for-the-badge)
![GitHub Workflow Status (with event)](https://img.shields.io/github/actions/workflow/status/maxgio92/capsule-addon-fluxcd/scan-code.yml?style=for-the-badge&label=GoSec)

# Capsule addon for Flux CD

This addon enables smooth integration of multi-tenancy in Kubernetes with [Capsule](https://capsule.clastix.io/), the GitOps-way with [Flux CD](https://fluxcd.io/).

In particular enables `Tenant`s to manage their resources, including creating `Namespace`s, respecting the Flux [multi-tenancy lockdown](https://fluxcd.io/flux/installation/configuration/multitenancy/).

Tenant resources, represented as `Kustomization` / `HelmRelease` / etc. can be reconciled as Tenant owners.

This way tenants can be provided Namespace-as-a-Service in a GitOps fashion.

## Install

```shell
helm install -n capsule-system capsule-addon-fluxcd oci://ghcr.io/maxgio92/charts/capsule-addon-fluxcd
```

## How it works

With the addon, you as platform admin, for the *oil* `Tenant` just need a `ServiceAccount` with the `capsule.addon.fluxcd/enabled=true` annotation:

```yml
---
apiVersion: v1
kind: Namespace
metadata:
  name: oil-system
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: gitops-reconciler
  namespace: oil-system
  annotations:
    capsule.addon.fluxcd/enabled: "true"
```

and set it as a valid *oil* `Tenant` owner:

```yml
---
apiVersion: capsule.clastix.io/v1beta2
kind: Tenant
metadata:
  name: oil
spec:
  additionalRoleBindings:
  - clusterRoleName: cluster-admin
    subjects:
    - name: gitops-reconciler
      kind: ServiceAccount
      namespace: oil-system
  owners:
  - name: system:serviceaccount:oil-system:gitops-reconciler
    kind: ServiceAccount
---
apiVersion: capsule.clastix.io/v1beta2
kind: CapsuleConfiguration
metadata:
  name: default
spec:
  userGroups:
  - capsule.clastix.io
  - system:serviceaccounts:oil-system
```

Without the addon you would need to manually manage RBAC and kubeConfig for the Tenant owner.

The addon will automate the permissions and the `kubeConfig` `Secret` for the **ServiceAccount Tenant owner** in order to be used by Flux when reconciling Tenant resources.

Let's go through examples.

## Examples

Consider a `Tenant` named *oil* that has a dedicated Git repository that contains oil's configurations.

You want to provide to the *oil* `Tenant` a Namespace-as-a-Service with a GitOps experience, allowing the tenant to version the configurations in a Git repository.

You, as platform admin and Tenant owner, can configure Flux [reconciliation](https://fluxcd.io/flux/concepts/#reconciliation) resources to be applied as Tenant owner:

```yml
---
apiVersion: kustomize.toolkit.fluxcd.io/v1beta2
kind: Kustomization
metadata:
  name: oil-apps
  namespace: oil-system
spec:
  serviceAccountName: gitops-reconciler
  kubeConfig:
    secretRef:
      name: gitops-reconciler-kubeconfig
      key: kubeconfig
  sourceRef:
    kind: GitRepository
    name: oil
---
apiVersion: source.toolkit.fluxcd.io/v1beta2
kind: GitRepository
metadata:
  name: oil
  namespace: oil-system
spec:
  url: https://github.com/oil/oil-apps
```

Let's analyze the setup field by field:
- the `GitRepository` and the `Kustomization` are in a Tenant system `Namespace`
- the `Kustomization` refers to a `ServiceAccount` to be impersonated when reconciling the resources the `Kustomization` refers to: this ServiceAccount is a *oil* **Tenant owner**
- the `Kustomization` refers also to a `kubeConfig` to be used when reconciling the resources the `Kustomization` refers to: this is needed to make requests through the **Capsule proxy** in order to operate on cluster-wide resources as a Tenant

The *oil* tenant can also declare new `Namespace`s thanks to the segregation provided by Capsule.

> Note: it can be avoided to explicitely set the the service account name when it's set as default Service Account name at Flux's [kustomize-controller level](https://fluxcd.io/flux/installation/configuration/multitenancy/#how-to-configure-flux-multi-tenancy) via the `default-service-account` flag.

## Additional features

### kubeConfig across Tenant Namespaces

The addon can also automate the distribution of the Tenant owner's kubeConfig `Secret` across all Tenant's `Namespaces`s.

This is implemented with Capsule's `GlobalTenantResource` custom resource.

You just need to add the annotation `capsule.addon.fluxcd/kubeconfig-global=true` to the Tenant owner `ServiceAccount`.

## Documentation

More information in the Capsule official guide [Multi-tenancy the GitOps way](https://capsule.clastix.io/docs/guides/flux2-capsule/#the-ingredients-of-the-recipe).

## Development

### Linting

```shell
make lint
```

### End-to-end testing

```shell
make e2e
```
