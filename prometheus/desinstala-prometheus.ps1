# desinstala prometheus stack y los objetos kubernetes relacionados
helm uninstall kube-prom-stack --namespace monitoring --wait

# borra recursos de kubernetes
kubectl delete -f ./mi-app-monitoring.yaml --namespace monitoring --ignore-not-found
kubectl delete -f ./grafana-secret.yaml --namespace monitoring --ignore-not-found

# Elimina PVCs y PVs asociados a Grafana en el namespace monitoring
Write-Host "Deleting Grafana PVCs in monitoring namespace (if any)..."
kubectl delete pvc -l app.kubernetes.io/name=grafana -n monitoring --ignore-not-found

# Use PowerShell JSON handling to find PVs whose claimRef.namespace == monitoring and delete them
$pvJson = kubectl get pv -o json
if ($?) {
	$pvObj = $pvJson | ConvertFrom-Json
	$pvObj.items | Where-Object { $_.spec.claimRef -and $_.spec.claimRef.namespace -eq "monitoring" } | ForEach-Object {
		$pvName = $_.metadata.name
		Write-Host "Deleting PV: $pvName"
		kubectl delete pv $pvName --ignore-not-found
	}
}

# elimina el repo
helm repo remove prometheus-community

# Elimina el namespace monitoring (si existe)
kubectl delete namespace monitoring --ignore-not-found
