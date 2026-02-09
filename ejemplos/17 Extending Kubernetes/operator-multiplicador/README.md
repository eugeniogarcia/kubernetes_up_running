# Multiplicador operator (example)

This is a minimal example operator that watches a CustomResource `Multiplicador` and reconciles a Deployment running `docker.io/egsmartin/multiplica:latest`.

Key ideas demonstrated:
- A CRD definition for `Multiplicador`.
- A simple controller (polling reconciler) implemented in Go using the dynamic client and the core clientset.
- The operator ensures a Deployment exists with `replicas` and `MULTIPLIER` (env) taken from the CR spec.
- Basic validation: if `spec.multiplier` <= 0 we annotate the CR and skip creating/updating the Deployment.

Quick deploy steps (assumes `kind` + `kubectl` + `docker`):

1) Apply the CRD:

```bash
kubectl apply -f k8s/multiplicador-crd.yaml
```

2) Build & load operator image into kind (from repo root):

```bash
docker build -t multiplicador-operator:local ./operator-multiplicador
kind load docker-image multiplicador-operator:local
```

3) Run the operator in the cluster:

```bash
kubectl apply -f k8s/operator-deployment.yaml
```

4) Create sample Multiplicador:

```bash
kubectl apply -f k8s/multiplicador-sample.yaml
```

5) Inspect created Deployment and service:

```bash
kubectl get deployments
kubectl get pods
kubectl get multiplicadors -A
kubectl logs deployment/multiplicador-operator -n default
```
