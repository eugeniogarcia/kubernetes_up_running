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

actualizamos el cluster de temporal con las propiedades de persistencia:

```ps
helm get values temporal -n mitemporal > current-values.yaml

helm upgrade temporal temporal/temporal --namespace mitemporal -f temporal.yaml

helm upgrade temporal temporal/temporal --namespace mitemporal -f current-values.yaml

```

```
tctl --namespace mitemporal namespace update \
  --history_archival_state enabled \
  --history_archival_uri s3://temporal-history \
  --visibility_archival_state enabled \
  --visibility_archival_uri s3://temporal-visibility \
  --retention 48h
```

```
tctl --namespace default namespace update \
  --history_archival_state enabled \
  --history_archival_uri s3://temporal-history \
  --visibility_archival_state enabled \
  --visibility_archival_uri s3://temporal-visibility \
  --retention 48h
```
