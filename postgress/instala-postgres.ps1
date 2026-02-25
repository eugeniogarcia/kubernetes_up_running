# Setup repositorio CloudNativePG 
Write-Host "Añade el repo helm de CloudNativePG ..."
helm repo add cnpg https://cloudnative-pg.github.io/charts
helm repo update

# Instala el operador de CloudNativePG
Write-Host "Instala el operador de CloudNativePG en el namespace 'cloudnative-pg'..."
helm install cloudnative-pg cnpg/cloudnative-pg --namespace cloudnative-pg --create-namespace

# Espera hasta que el webhook del operador esté listo (tiene endpoints)
Write-Host "Esperando a que el webhook del operador esté listo (timeout 10m)..."
$timeout = 600
$interval = 5
$elapsed = 0
while ($true) {
	$ep = & kubectl -n cloudnative-pg get endpoints cnpg-webhook-service -o jsonpath='{.subsets}' 2>$null
	if ($LASTEXITCODE -eq 0 -and $ep -and $ep -ne "") {
		Write-Host "Webhook endpoints detectados."
		break
	}
	if ($elapsed -ge $timeout) {
		Write-Host "Timeout esperando webhook. Continúo pero la creación del Cluster puede fallar si el webhook no está listo."
		break
	}
	Start-Sleep -Seconds $interval
	$elapsed += $interval
}

# Instala la clase de almacenamiento dinamico local-path
Write-Host "Ensure local-path storage class (if you already have it, this is a no-op)..."
kubectl apply -f https://raw.githubusercontent.com/rancher/local-path-provisioner/master/deploy/local-path-storage.yaml

# descarga la imagen busybox para que el operador pueda usarla en los init containers 
docker pull busybox

# Crea la Namespace y Secret, luego el Cluster CR (el operador y el cluster están en namespaces distintos)
Write-Host "Creando Namespace y Secret en 'database'..."
kubectl apply -f .\database-namespace-secret.yaml
Write-Host "Creando el Cluster CR en namespace 'database'..."
kubectl apply -f .\cnpg-cluster.yaml

# Esperamos a que el cluster de postgres este listo
Write-Host "Esperando a que el cluster de postgres este listo (timeout 10m)..."
kubectl -n database wait --for=condition=Ready cluster/mi-postgres --timeout=600s

# Mensajes finales
Write-Host "Operación terminada. Par ver los objetos kubernetes creados:"
Write-Host "  kubectl -n database get pods,svc,pvc,secret"
Write-Host "  kubectl -n database get cluster mi-postgres -o wide"
