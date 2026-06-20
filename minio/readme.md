# MinIO + Temporal Archival en Kubernetes

## 📌 ¿Qué es MinIO?

MinIO es una plataforma de almacenamiento de objetos **compatible al 100% con la API de Amazon S3**, diseñada para entornos **on‑premise**, **Kubernetes** y **cloud híbrida**.

MinIO ofrece:
- Almacenamiento distribuido y replicado
- Alta disponibilidad
- API S3 estándar
- Rendimiento muy alto
- Integración nativa con aplicaciones que esperan un backend S3 (como Temporal)

---

## 📌 ¿Por qué usar MinIO con Temporal?

Temporal requiere un backend de almacenamiento para **Archival**, donde guarda:

- Historias de workflows cerrados
- Registros de visibilidad
- Datos que deben persistir más allá del retention del namespace

Temporal **solo soporta oficialmente backends S3/GCS**.  
Si no usas AWS, la alternativa profesional es **MinIO**, que expone la misma API S3.

Esto permite:

- Activar `history_archival`
- Activar `visibility_archival`
- Configurar retención (por ejemplo, 48h)
- Consultar workflows archivados incluso después de ser limpiados de la base de datos

---

## 📌 ¿Por qué no sirve `local-path-provisioner`?

`local-path-provisioner` (que hemos usado conel setup de postgres) crea volúmenes locales ligados a un único nodo. Esto **no es válido** para archival porque:

- No replica datos
- No es accesible desde todos los pods
- No garantiza durabilidad
- No soporta la arquitectura distribuida de Temporal

Por eso se necesita un backend S3 real o compatible → **MinIO**.

## Instalación

```ps
helm repo add minio https://charts.min.io/
helm repo update
```

creamos el servicio en  kubernetes

```ps
kubectl create namespace minio

helm install minio minio/minio --namespace minio -f values.yaml
```

**Nota**: Los recursos que se crean con el chart los podemos ver resumidos en `recursos.yaml`.

verificar:

```ps
kubectl get pods -n minio
kubectl get svc -n minio
```

acceder a la consola con `minio` y `minio123`:

```ps
kubectl port-forward -n minio svc/minio-console 9001:9001
```

actualizamos el cluster de temporal con las propiedades de persistencia. En primer lugar, como ya tenemos el cluster temporal.io creado, obtenemos la configuración actual:

```ps
helm get values temporal -n mitemporal > current-values.yaml
```

Sobre esa configuración, actualizamos los valores indicados en `temporal.yaml`, y con el archivo resultante hacemos la actualización del chart:

```ps
helm upgrade temporal temporal/temporal --namespace mitemporal -f current-values.yaml
```

- con esto hemos definido las rutas para el archival, `history` (guarda la historia del workflow: cada evento, señal, actividad iniciada/terminada, timer, flujo hijo, etc.) y `visibility` (guarda los metadatos asociados al workflow: runid, start and end times, side-effects, ...)
- y hemos habilitado por defecto el archival para los namespaces (la configuración de rutas es común para todos los namespaces)

Con el CLI podemos habilitar y deshabilitar el archival namespace a namespace (aunque las rutas serán las mismas para todos los namespaces habilitados). En primer lugar nos conectamos al Pod donde están las admin-tools:

```ps
kubectl exec -it -n mitemporal temporal-admintools-57554876c6-qlhfz -- sh
```

y ya podemos proceder a habilitar el archiving:

```sh
temporal operator namespace update --history-archival-state enabled -n default

temporal operator namespace update --visibility-archival-state enabled -n default

temporal operator namespace describe -n default
```

podemos fijar el tiempo de retención:

```sh
temporal operator namespace update -n default --retention 1d
```

Los datos archivados se guardarán en los dos buckets de s3 que tenemos en minio

**Podemos diagnosticar errores al aplicar los comandos de `temporal operator ...` observando los logs**:

```ps
kubectl logs deploy/temporal-frontend -n mitemporal
```