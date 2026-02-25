# Script para deshacer la instalación realizada por `instala-postgres.ps1`.
# Ejecutar desde la raíz del repo en PowerShell.

Write-Host "Desinstalando release Helm 'cloudnative-pg' (si existe)..."
try {
	helm uninstall cloudnative-pg -n cloudnative-pg
} catch {
	Write-Host "Release no encontrada o ya eliminada."
}

Write-Host "Eliminando namespace 'cloudnative-pg' (si existe)..."
kubectl delete namespace cloudnative-pg --ignore-not-found

Write-Host "Eliminando Cluster CR en namespace 'database' (si existe)..."
kubectl delete -f .\cnpg-cluster.yaml --ignore-not-found
Write-Host "Eliminando Namespace y Secret 'database' (si existen)..."
kubectl delete -f .\database-namespace-secret.yaml --ignore-not-found

Write-Host "Eliminando StorageClass local-path (aplicación instalada desde URL)..."
kubectl delete -f https://raw.githubusercontent.com/rancher/local-path-provisioner/master/deploy/local-path-storage.yaml --ignore-not-found
