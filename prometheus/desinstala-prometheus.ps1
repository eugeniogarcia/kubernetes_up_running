# desinstala prometheus stack y los objetos kubernetes relacionados
helm uninstall kube-prom-stack --namespace monitoring --wait

# borra recursos de kubernetes
kubectl delete -f ./mi-app-monitoring.yaml --namespace monitoring --ignore-not-found
kubectl delete -f ./grafana-secret.yaml --namespace monitoring --ignore-not-found

# elimina el repo
helm repo remove prometheus-community

# Elimina el namespace monitoring (si existe)
kubectl delete namespace monitoring --ignore-not-found
