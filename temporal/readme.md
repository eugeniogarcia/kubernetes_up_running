## Intro

Temporal uses the following pods & services:

- `web`: la UI web.
- `frontend`: la API pública que usan los SDKs y la CLI.
- `history`: gestiona el estado e historial de workflows.
- `matching`: maneja colas de tareas.
- `worker`: hace trabajo interno de coordinación.
- `admintools`: contenedor para tareas de esquema, bootstrap y CLI.

Temporal requires persistence stores for:
- **Default store**: Stores workflow execution data (history, tasks, etc.)
- **Visibility store**: Stores workflow visibility/search data

The persistence configuration follows the raw Temporal server config format. Configure it under `server.config.persistence.datastores`:

```yaml
server:
  config:
    persistence:
      defaultStore: default
      visibilityStore: visibility
      numHistoryShards: 512
      datastores:
        default:
          sql:
            createDatabase: true
            manageSchema: true
            pluginName: mysql8  # or postgres12, postgres12_pgx
            driverName: mysql8
            databaseName: temporal
            connectAddr: "mysql.example.com:3306"
            connectProtocol: tcp
            user: temporal_user
            # Option 1: Provide password in values (chart will create a secret)
            password: your_password
            # Option 2: Use an existing secret (recommended for production)
            # existingSecret: temporal-db-secret
            # secretKey: password
            maxConns: 20
            maxIdleConns: 20
            maxConnLifetime: "1h"
        visibility:
          sql:
            createDatabase: true
            manageSchema: true
            pluginName: mysql8
            driverName: mysql8
            databaseName: temporal_visibility
            connectAddr: "mysql.example.com:3306"
            connectProtocol: tcp
            user: temporal_user
            # Use existing secret (recommended for production)
            existingSecret: temporal-db-secret
            secretKey: password
```

**Key points:**
- Driver is determined by which key is present (`sql:`, `cassandra:`, or `elasticsearch:`)
- **Helm-specific fields** (stripped before rendering to server config):
  - `createDatabase`: If `true`, the chart will create the database/keyspace if it doesn't exist (default: `true`)
  - `manageSchema`: If `true`, the chart will run schema setup/upgrade jobs (default: `true`)
  - `existingSecret`: Reference to an existing Kubernetes secret containing credentials (e.g., `temporal-db-secret`). If not set, the chart will create a new secret.
  - `secretKey`: Key name within the secret to read the password from (default: `password`)
- **Password handling**: With `password` or `existingSecret`, passwords are stored in Kubernetes secrets and read from environment variables—they are never written to ConfigMaps or other manifests, even if you supply a plaintext `password` in values for bootstrap only.
  - If `existingSecret` is set, the chart uses that secret and ignores any `password` field in values for that datastore
  - If `existingSecret` is not set, the chart creates a secret from the `password` value in values
  - The server configuration reads passwords from environment variables that reference these secrets
  - As a third option, `passwordCommand` (Temporal server v1.31+, **SQL datastores only**) lets the server invoke a shell command per new connection to fetch the password — handy for short-lived credentials such as AWS RDS IAM auth tokens or GCP Cloud SQL IAM tokens. When set on a datastore, the chart skips creating a password Secret and skips wiring the `*_PASSWORD` env var for that store; the `passwordCommand` block passes through to the server config as-is.
- All other fields pass through directly to the Temporal server config

## Using an existing Kubernetes secret

For production and GitOps, manage database credentials in a Kubernetes `Secret` that you (or a controller such as External Secrets) create and own outside this chart. Point each datastore at that object with `existingSecret` (the secret name) and `secretKey` (the key inside the secret that holds the password; default `password`).

The secret must exist in the same namespace as the release before the chart’s jobs or pods need it. A typical manifest looks like this (`stringData` is fine if you prefer not to base64-encode by hand):

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: temporal-db-secret
type: Opaque
data:
  password: <base64-encoded-password>
```

Reference it from your values (here both stores share one secret; use separate secrets if you split credentials):

```yaml
server:
  config:
    persistence:
      datastores:
        default:
          sql:
            pluginName: postgres12_pgx
            driverName: postgres12_pgx
            databaseName: temporal
            connectAddr: "postgres.example.com:5432"
            connectProtocol: tcp
            user: temporal_user
            existingSecret: temporal-db-secret
            secretKey: password
        visibility:
          sql:
            pluginName: postgres12_pgx
            driverName: postgres12_pgx
            databaseName: temporal_visibility
            connectAddr: "postgres.example.com:5432"
            connectProtocol: tcp
            user: temporal_user
            existingSecret: temporal-db-secret
            secretKey: password
```

For a disposable local cluster only, you can seed a minimal secret before `helm install` with `kubectl create secret generic temporal-db-secret --from-literal=password=your_db_password`. Prefer your standard secret workflow everywhere else.

## Install with PostgreSQL

To use a PostgreSQL database, copy the [PostgreSQL values file](values/values.postgresql.yaml) locally and edit it with your database connection details:

```yaml
server:
  config:
    persistence:
      datastores:
        default:
          sql:
            createDatabase: true
            manageSchema: true
            pluginName: postgres12
            driverName: postgres12
            databaseName: temporal
            connectAddr: "postgres.example.com:5432"
            connectProtocol: tcp
            user: temporal_user
            existingSecret: temporal-db-secret
            secretKey: password
        visibility:
          sql:
            createDatabase: true
            manageSchema: true
            pluginName: postgres12
            driverName: postgres12
            databaseName: temporal_visibility
            connectAddr: "postgres.example.com:5432"
            connectProtocol: tcp
            user: temporal_user
            existingSecret: temporal-db-secret
            secretKey: password
```

we have to ensure that the user - and credential - we have specified is a valid user in Postgres, and that, in case we are to create the schema in the db, that the user has the create db role, otherwise grant it:

```bash
kubectl exec -it -n database mi-postgres-1 -- bash
```

and then

```bash
psql

ALTER ROLE egsmartin CREATEDB;

\du
```

helm repo list

```bash
helm repo list
```

```bash
helm repo add temporal https://go.temporal.io/helm-charts
helm repo update
```

borrar un repo

```bash
helm repo remove temporal
```

creamos el namespace y el secret::

```bash
kubectl apply -f temporal-namespace-secret.yaml
```

insalamos el chart:

```bash
helm install temporal temporal/temporal -n mitemporal -f temporal.yaml --wait --timeout 15m
```

lo podemos actulizar:

```bash
helm upgrade temporal temporal/temporal -n mitemporal -f temporal.yaml --wait --timeout 15m
```

y para borrar:

```bash
helm uninstall temporal -n mitemporal
```

las admintols se pueden instalar opcionalmente si incluimos:

```yaml
# instalamos las admin tools para poder ejecutar comandos de temporal desde el cluster
admintools:
  enabled: true
  image:
    repository: temporalio/admin-tools
    tag: latest
```

con estas tools podemos operar con Temporal. Por ejemplo, podríamos crear un namespace:

```bash
kubectl exec -it -n mitemporal temporal-admintools-57554876c6-gp7jz -- temporal operator namespace create -n default
```

NOTA: Hemos incluido en los valores del chart que se cree ya el namespace por defecto

## Archival

Tenemos una descripción de [como usar minio](https://github.com/eugeniogarcia/kubernetes_up_running/blob/main/readme.md) para configurar el archival de temporal.io. Con minio emulamos la api de AWS s3 para proporcionar almancenamiento. Temporal.io soporta solo AWS o GCP para el almacenamiento para archival.
