## ¿Qué es Helm y cómo funciona un chart?

Helm es un gestor de paquetes para Kubernetes. Un *chart* es un paquete versionado que contiene plantillas (templates) de los manifiestos Kubernetes y los valores por defecto. Helm permite instalar, actualizar y desinstalar conjuntos de recursos como una única unidad (un *release*).

- ¿Para qué sirve Helm?:
	- Empaquetar y distribuir aplicaciones Kubernetes.
	- Parametrizar despliegues con `values.yaml` y `--set`/`-f` en la línea de comando.
	- Llevar historial de versiones del release y permitir rollbacks.

### Estructura básica de un chart

- `Chart.yaml`: metadatos del chart (nombre, versión, descripción, icon, appVersion).
- `values.yaml`: valores por defecto que usan las plantillas. Son los valores que puedes sobreescribir con `--set` o `-f`.
- `templates/`: carpeta con plantillas Go-templates que generan los manifiestos Kubernetes (Deployment, Service, Ingress, etc.).
- `templates/_helpers.tpl`: funciones helper/partials para reutilizar fragmentos.
- `charts/`: subcharts (dependencias) si los hubiera.
- `templates/tests/`: pruebas para el chart (opcional).

### Flujo típico para crear y usar un chart

1. Crear la estructura (puedes usar `helm create <name>` o crearla manualmente).
2. Definir valores por defecto en `values.yaml`.
3. Escribir plantillas en `templates/` usando `{{ .Values }}` para acceder a parámetros.
4. Probar localmente con `helm lint ./mi-chart` y `helm template ./mi-chart`.
5. Instalar: `helm install mi-chart ./mi-chart --values ./mi-chart/values.yaml`.
6. Actualizar: `helm upgrade mi-chart ./mi-chart --reuse-values --set ...`.
7. Desinstalar: `helm uninstall mi-chart`.

en la carpeta `mi-chart` hay definido un chart. Vamos a utilizar este ejemplo para demostrar como operar con el chart y helm.

## Operar con un chart

Podemos validar que el chart este bien:

```ps
helm lint ./mi-chart
```

si esta todo bien procedemos a instalar el chart:

```ps
helm install mi-chart ./mi-chart --values ./mi-chart/values.yaml
```

la salida es:

```ps
NAME: mi-chart
LAST DEPLOYED: Tue Feb 24 08:44:29 2026
NAMESPACE: default
STATUS: deployed
REVISION: 1
DESCRIPTION: Install complete
TEST SUITE: None
```

Para ver el estado de todos los artefactos creados:

```ps
helm status mi-chart
```

la salida es:

```ps
NAME: mi-chart
LAST DEPLOYED: Tue Feb 24 08:44:29 2026
NAMESPACE: default
STATUS: deployed
REVISION: 1
DESCRIPTION: Install complete
RESOURCES:
==> v1/Service
NAME   TYPE        CLUSTER-IP     EXTERNAL-IP   PORT(S)   AGE
mult   ClusterIP   10.96.181.99   <none>        80/TCP    65s
primos   ClusterIP   10.96.95.62   <none>   80/TCP   65s

==> v1/Deployment
NAME   READY   UP-TO-DATE   AVAILABLE   AGE
mult   2/2     2            2           65s
primos   2/2   2     2     65s

==> v1/Pod(related)
NAME                   READY   STATUS    RESTARTS   AGE
mult-ff596ff5c-7lp8t   1/1     Running   0          65s
mult-ff596ff5c-qzs75   1/1     Running   0          65s
primos-7f77f54f4f-6pwzk   1/1   Running   0     65s
primos-7f77f54f4f-9l8bn   1/1   Running   0     65s

==> v1/Ingress
NAME             CLASS     HOSTS    ADDRESS      PORTS   AGE
simple-ingress   contour   gz.com   172.18.0.6   80      65s

==> v1/HTTPProxy
NAME          FQDN          TLS SECRET   STATUS   STATUS DESCRIPTION
operaciones   oper.gz.com                valid    Valid HTTPProxy
```

podemos ver todos los artefactos que se han creado. 

Cuando hemos instalado el chart, la información se guarda en el cluster - en un secret. Si cambiamos los valores de los artefactos kubernetes directamente habrá un mismatch entre la definición del chart y lo que realmente se ejecuta. Este mismatch no entra en el bucle de reconciliación, pero si más adelante hacemos un `helm upgrade`, se volverán a aplicar los parámetros registrados con el chart, por este motivo si por ejemplo queremos escalar el número de pods es recomendable hacerlo sobre el chart directamente. Para aplicar el cambio sobre el chart haríamos:

```ps
helm upgrade mi-chart ./mi-chart --reuse-values --set mult.replicaCount=3
```

esto lo que hace es tomar los valores del chart, los valores por defecto, y aplicar los parametros indicados. Si NO usamos `reuse-values`, lo que se toman son los últimos valores aplicados al chart - los que están en el cluster -, y aplicar sobre ellos los cambios.

por ultimo, podemos eliminar el chart:

```ps
helm uninstall mi-chart -n default
```
