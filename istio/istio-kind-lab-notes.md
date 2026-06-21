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
helm upgrade istiod istio/istiod -n istio-system --wait -f .\outputs\istio-istiod-values.yaml
kubectl rollout restart deployment/istiod -n istio-system
```

## Desplegar el laboratorio

```powershell
kubectl apply -f .\lab-observabilidad.yaml
kubectl apply -f .\lab-workloads.yaml
kubectl apply -f .\lab-otros.yaml

kubectl wait --for=condition=available deployment/hello-v1 -n istio-lab --timeout=120s
kubectl wait --for=condition=available deployment/hello-v2 -n istio-lab --timeout=120s
kubectl wait --for=condition=available deployment/client -n istio-lab --timeout=120s
kubectl wait --for=condition=available deployment/opentelemetry-collector -n observability --timeout=120s
```

## Comprobaciones

Ver sidecars inyectados:

```powershell
kubectl get pod -n istio-lab -o jsonpath="{range .items[*]}{.metadata.name}{': '}{range .spec.containers[*]}{.name}{' '}{end}{'\n'}{end}"
kubectl get pod -n istio-legacy -o jsonpath="{range .items[*]}{.metadata.name}{': '}{range .spec.containers[*]}{.name}{' '}{end}{'\n'}{end}"
```

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

mTLS estricto y AuthorizationPolicy:

```powershell
# Debe funcionar: cliente con sidecar y service account autorizada.
kubectl exec -n istio-lab $CLIENT -c curl -- curl -s -o /dev/null -w "%{http_code}`n" http://hello

# Debe fallar o devolver 000/connection reset: namespace sin sidecar hacia destino STRICT mTLS.
$LEGACY = kubectl get pod -n istio-legacy -l app=legacy-client -o jsonpath="{.items[0].metadata.name}"
kubectl exec -n istio-legacy $LEGACY -c curl -- curl -s -m 5 -o /dev/null -w "%{http_code}`n" http://hello.istio-lab.svc.cluster.local
```

Trazas OpenTelemetry:

```powershell
1..5 | ForEach-Object { kubectl exec -n istio-lab $CLIENT -c curl -- curl -s http://hello | Out-Null }
kubectl logs -n observability deploy/opentelemetry-collector --tail=120
```

Deberias ver spans exportados por Envoy/Istio en el exporter `debug` del collector.

## Limpieza

```powershell
kubectl delete -f .\lab-observabilidad.yaml
kubectl delete -f .\lab-workloads.yaml
kubectl delete -f .\lab-otros.yaml

helm uninstall istio-ingress -n istio-ingress
helm uninstall istiod -n istio-system
helm uninstall istio-base -n istio-system
kubectl delete namespace istio-system istio-ingress
```
