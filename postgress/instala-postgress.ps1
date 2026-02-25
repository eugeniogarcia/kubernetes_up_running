# Setup repositorio CloudNativePG 
Write-Host "Añade el repo helm de CloudNativePG ..."
helm repo add cnpg https://cloudnative-pg.github.io/charts
helm repo update

# Instala el operador de CloudNativePG
Write-Host "Instala el operador de CloudNativePG en el namespace 'cloudnative-pg'..."
helm install cloudnative-pg cnpg/cloudnative-pg --namespace cloudnative-pg --create-namespace

# Instala la clase de almacenamiento dinamico local-path
Write-Host "Ensure local-path storage class (if you already have it, this is a no-op)..."
kubectl apply -f https://raw.githubusercontent.com/rancher/local-path-provisioner/master/deploy/local-path-storage.yaml

# Crea los objetos kubernetes, namespace, secret y Cluster CR
Write-Host "Crea objetos kubernetes, namespace, secret y Cluster CR..."
kubectl apply -f .\postgress\cnpg-cluster.yaml

# Esperamos a que el cluster de postgres este listo
Write-Host "Esperando a que el cluster de postgres este listo (timeout 10m)..."
kubectl -n database wait --for=condition=Ready cluster/mi-postgres --timeout=600s

# Mensajes finales
Write-Host "Operación terminada. Par ver los objetos kubernetes creados:"
Write-Host "  kubectl -n database get pods,svc,pvc,secret"
Write-Host "  kubectl -n database get cluster mi-postgres -o wide"
