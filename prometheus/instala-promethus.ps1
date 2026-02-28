#crea el secret con las credenciales de grafana
kubectl apply -f ./grafana-secret.yaml

# instala repo
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

# instala chart de prometheus en el namespace monitoring (se crea si no existe)
helm install kube-prom-stack prometheus-community/kube-prometheus-stack --namespace monitoring --create-namespace -f values.yaml

# crea los objetos kubernetes
kubectl apply -f ./mi-app-monitoring.yaml