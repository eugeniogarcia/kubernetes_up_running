#crea el secret con las credenciales de grafana
kubectl apply -f ./grafana-secret.yaml

# instala repo
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

# Instala la clase de almacenamiento dinamico local-path
Write-Host "Ensure local-path storage class (if you already have it, this is a no-op)..."
kubectl apply -f https://raw.githubusercontent.com/rancher/local-path-provisioner/master/deploy/local-path-storage.yaml

# instala chart de prometheus en el namespace monitoring (se crea si no existe)
helm install kube-prom-stack prometheus-community/kube-prometheus-stack --namespace monitoring --create-namespace -f values.yaml

# crea los objetos kubernetes
kubectl apply -f ./mi-app-monitoring.yaml
