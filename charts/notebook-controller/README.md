# notebook-controller

This chart installs the Notebook CRD and the Notebook Controller as a standalone
component. It does not require `ack-ai-dev-console`.

## Install

```bash
helm install notebook-controller ./charts/notebook-controller \
  --namespace notebook-controller-system \
  --create-namespace
```

## Verify

```bash
kubectl get deployment notebook-controller-deployment \
  --namespace notebook-controller-system
kubectl get crd notebooks.kubeflow.org
```

The controller watches `Notebook` resources in all namespaces. Only one release
of this chart should be installed in a cluster because its CRD and ClusterRole
resources are cluster-scoped.

## Uninstall

```bash
helm uninstall notebook-controller --namespace notebook-controller-system
```

Helm does not remove CRDs from a chart's `crds/` directory. Delete the Notebook
CRD separately only when all Notebook resources are no longer needed.
