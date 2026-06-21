# Istio en kind: laboratorio rapido

## Instalacion con Helm

```powershell
helm repo add istio https://istio-release.storage.googleapis.com/charts

helm repo update

# CRD de Istio: VirtualService, DestinationRule, PeerAuthentication, AuthorizationPolicy, Gateway, Sidecar, Telemetry, etc
helm install istio-base istio/base -n istio-system --create-namespace --set defaultRevision=default

# istiod: plano de control
helm install istiod istio/istiod -n istio-system --wait -f .\istio-istiod-values.yaml

# Opcional si quieres exponer entrada desde fuera del cluster.
helm install istio-ingress istio/gateway -n istio-ingress --create-namespace --wait
```

Si ya tenias `istiod` instalado, aplica los valores de tracing con:

```powershell
helm upgrade istiod istio/istiod -n istio-system --wait -f .\istio-istiod-values.yaml
kubectl rollout restart deployment/istiod -n istio-system
```

## Desplegar el laboratorio

```powershell
kubectl apply -f .\lab-workloads.yaml
kubectl apply -f .\lab-otros.yaml

kubectl wait --for=condition=available deployment/hello-v1 -n istio-lab --timeout=120s
kubectl wait --for=condition=available deployment/hello-v2 -n istio-lab --timeout=120s
kubectl wait --for=condition=available deployment/client -n istio-lab --timeout=120s
kubectl wait --for=condition=available deployment/opentelemetry-collector -n observability --timeout=120s
```

## Comprobaciones

### Sidecars

Ver sidecars inyectados:

```powershell
kubectl get pod -n istio-lab -o jsonpath="{range .items[*]}{.metadata.name}{': '}{range .spec.containers[*]}{.name}{' '}{end}{'\n'}{end}"
kubectl get pod -n istio-legacy -o jsonpath="{range .items[*]}{.metadata.name}{': '}{range .spec.containers[*]}{.name}{' '}{end}{'\n'}{end}"
```

podemos ver que en el namespace que hemos creado para los workloads, `istio-lab`, tiene habilitada la inyeccion de sidecars:

```powershell
kubectl get ns istio-lab --show-labels

NAME        STATUS   AGE   LABELS
istio-lab   Active   13m   istio-injection=enabled,kubernetes.io/metadata.name=istio-lab
```

Si tenemos que cambiar la etiqueta/inyección, podemos hacer::

```powershell
kubectl label namespace istio-lab istio-injection=enabled --overwrite
```

seguido de:

```ps
kubectl rollout restart deployment -n istio-lab
kubectl rollout status deployment/client -n istio-lab
kubectl rollout status deployment/hello-v1 -n istio-lab
kubectl rollout status deployment/hello-v2 -n istio-lab
```

```ps
get pod -n istio-lab

NAME                        READY   STATUS    RESTARTS   AGE
client-6d6d7c8945-q9vxj     2/2     Running   0          18m
hello-v1-758b5c44f5-kwg7w   2/2     Running   0          18m
hello-v2-6bb8cd565f-9nlhj   2/2     Running   0          18m
```

```powershell
kubectl get pods -n istio-system

NAME                      READY   STATUS    RESTARTS   AGE
istiod-695fd89f4f-sqvph   1/1     Running   0          166m

kubectl get mutatingwebhookconfiguration

NAME                                  WEBHOOKS   AGE
cnpg-mutating-webhook-configuration   4          10d
istio-sidecar-injector                4          167m
```

### Networking

Se ha creado un `VirtualService` que aplica a un par de hosts:

```yaml
hosts:
  - hello
  - hello.istio-lab.svc.cluster.local
```

con tres definiciones, dos de ellas se activan en funcion de la presencia de una cabecera y la tercera es por defecto:

- envia todas las peticiones al destino `hello.istio-lab.svc.cluster.local` subset `v2`, si la cabecera `x-demo-version` vale `v2`:

```yaml
- name: header-route-to-v2
  match:
    - headers:
        x-demo-version:
          exact: v2
  route:
    - destination:
        host: hello.istio-lab.svc.cluster.local
        subset: v2
      weight: 100
```

- envia todas las peticiones al destino `hello.istio-lab.svc.cluster.local` subset `v1`, si la cabecera `x-demo-delay` vale `true`. El `100%` de las peticiones tendrá un retraso de dos segundos:

```yaml
- name: delay-demo
  match:
    - headers:
        x-demo-delay:
          exact: "true"
  fault:
    delay:
      percentage:
        value: 100
      fixedDelay: 2s
  route:
    - destination:
        host: hello.istio-lab.svc.cluster.local
        subset: v1
      weight: 100
```

- y por defecto envía todas las peticiones al destino `hello.istio-lab.svc.cluster.local`, el `90%` de ellas al subset `v1`, y el `10%` al subset `v2`:

```yaml
- name: weighted-split
  route:
    - destination:
        host: hello.istio-lab.svc.cluster.local
        subset: v1
      weight: 90
    - destination:
        host: hello.istio-lab.svc.cluster.local
        subset: v2
      weight: 10
```

el destino `hello.istio-lab.svc.cluster.local` está definido en una `DestinationRule`. Aquí se definen los dos subsets que hemos usado antes, el `v1` y `v2` (utilizando las `labels` para identificar los Pods destino), el circuit braker, pool de conexiones, la securización de la llamada (tls `ISTIO_MUTUAL`) y el throttling.

Llamada normal, con split 90/10 entre v1 y v2:

```powershell
$CLIENT = kubectl get pod -n istio-lab -l app=client -o jsonpath="{.items[0].metadata.name}"
1..20 | ForEach-Object { kubectl exec -n istio-lab $CLIENT -c curl -- curl -s http://hello }
```

Routing por cabecera:

```powershell
kubectl exec -n istio-lab $CLIENT -c curl -- curl -s -H "x-demo-version: v2" http://hello
```

Fault injection con delay de 2 segundos:

```powershell
kubectl exec -n istio-lab $CLIENT -c curl -- curl -s -w "time=%{time_total}`n" -H "x-demo-delay: true" http://hello
```

### Políticas

Por último se definen las políticas en el namespace:

```yaml
apiVersion: security.istio.io/v1
kind: PeerAuthentication
metadata:
  name: namespace-strict-mtls
  namespace: istio-lab
spec:
  mtls:
    mode: STRICT
```

se permiten llamadas solo desde `client` a `hello`:

```yaml
apiVersion: security.istio.io/v1
kind: AuthorizationPolicy
metadata:
  name: hello-only-from-client
  namespace: istio-lab
spec:
  selector:
    matchLabels:
      app: hello
  action: ALLOW
  rules:
    - from:
        - source:
            principals:
              - cluster.local/ns/istio-lab/sa/client
```

mTLS estricto y AuthorizationPolicy:

- Debe funcionar. Se llama desde `client` a `hello`. Cliente tiene el sidecar y una cuenta autorizada:

```powershell
kubectl exec -n istio-lab $CLIENT -c curl -- curl -s -o /dev/null -w "%{http_code}`n" http://hello
```

- Debe fallar o devolver 000/connection reset: desde `legacy-client` a `hello` tiene que fallar porque no tiene el sidecar inyectado, al no esta en el namespace.

```powershell
$LEGACY = kubectl get pod -n istio-legacy -l app=legacy-client -o jsonpath="{.items[0].metadata.name}"
kubectl exec -n istio-legacy $LEGACY -c curl -- curl -s -m 5 -o /dev/null -w "%{http_code}`n" http://hello.istio-lab.svc.cluster.local
```

## Monitorización

El setup que vamos a construir es el siguiente:

- **Métricas**: Istio sidecars -> Prometheus -> Thanos -> Grafana
- **Trazas**: Istio sidecars -> OpenTelemetry Collector -> Tempo -> Grafana
- **Logs**: Pod stdout logs -> OpenTelemetry Collector DaemonSet -> Loki -> Grafana
- **Configuracion de la mesh**: Istio mesh/config -> Kiali

Los helm charts que vamos a utilizar son:

```ps
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo add grafana https://grafana.github.io/helm-charts
helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo update
```

Orden de instalacion:

```powershell
helm upgrade --install thanos-minio bitnami/minio `
  -n observability --create-namespace `
  -f .\values-minio-thanos-kind.yaml

kubectl apply -f .\thanos-objstore-secret-kind.yaml

helm upgrade --install kps prometheus-community/kube-prometheus-stack `
  -n observability --create-namespace `
  -f .\values-kube-prometheus-stack.yaml

helm upgrade --install loki grafana/loki `
  -n observability `
  -f .\values-loki-kind.yaml

helm upgrade --install tempo grafana/tempo `
  -n observability `
  -f .\values-tempo-kind.yaml

helm upgrade --install otel-traces open-telemetry/opentelemetry-collector `
  -n observability `
  -f .\values-otel-traces-gateway.yaml

helm upgrade --install otel-logs open-telemetry/opentelemetry-collector `
  -n observability `
  -f .\values-otel-logs-daemonset.yaml

kubectl apply -f .\istio-telemetry-otel.yaml
kubectl apply -f .\istio-prometheus-monitors.yaml
```

URLs internas esperadas:

```text
Prometheus: http://kps-kube-prometheus-stack-prometheus.observability.svc:9090
Grafana:    http://kps-grafana.observability.svc:80
Loki:       http://loki-gateway.observability.svc.cluster.local
Tempo:      http://tempo.observability.svc.cluster.local:3100
OTel gRPC:  opentelemetry-collector.observability.svc.cluster.local:4317
```

Acceso local:

```powershell
kubectl port-forward -n observability svc/kps-grafana 3000:80
```

Abre:

```text
http://localhost:3000
```

Trazas OpenTelemetry:

```powershell
$CLIENT = kubectl get pod -n istio-lab -l app=client -o jsonpath="{.items[0].metadata.name}"

1..5 | ForEach-Object { kubectl exec -n istio-lab $CLIENT -c curl -- curl -s http://hello | Out-Null }

kubectl logs -n observability deploy/opentelemetry-collector --tail=120
```