nos conectamos al nodo

```powershell
kubectl exec -it mi-postgres-1 -n database -- bash
```

nos conectamos con el usuario administrador local:

```bash
psql -U postgres
```

o si nos queremos conectar directamente a una base de datos:

```bash
psql -U postgres -d "mi-db"
```

un ROLE en Postgres es un usuario. Podemos ver los usuarios:

```psql
\du

                                   List of roles
       Role name       |                         Attributes
-----------------------+------------------------------------------------------------
 cnpg_metrics_exporter |
 egsmartin             | Create DB
 postgres              | Superuser, Create role, Create DB, Replication, Bypass RLS
 streaming_replica     | Replication
 ```

tambien:

```psql
SELECT usename, usesuper, usecreatedb FROM pg_user;
        usename        | usesuper | usecreatedb
-----------------------+----------+-------------
 postgres              | t        | t
 cnpg_metrics_exporter | f        | f
 streaming_replica     | f        | f
 egsmartin             | f        | t
(4 rows)
```

podemos crear un usuario:

```psql
CREATE USER egsmartin WITH PASSWORD 'tu_contraseña';
```

cambiar la contraseña:

```psql
ALTER USER egsmartin WITH PASSWORD 'prueba';
```

listamos las bases de datos:

```psql
SELECT datname FROM pg_database WHERE datistemplate = false;

postgres
 mi-db
 temporal
 temporal_visibility
```

podemos crear una base de datos; Damos permisos y creamos la base de datos:

```psql
ALTER ROLE egsmartin CREATEDB;

CREATE DATABASE "mi-db";
```

podemos dar acceso a una base de datos:

```psql
GRANT CONNECT ON DATABASE "mi-db" TO egsmartin;
```

para que el usuario pueda interactuar con los objetos contenidos dentro de un esquema (generalmente public), debes darle permiso de uso:

```psql
-- Primero, asegúrate de estar conectado a la base de datos "mi-db":
\c "mi-db"

-- Otorga permisos para "usar" y buscar en el esquema public:
GRANT USAGE ON SCHEMA public TO egsmartin;

-- Nos permite conectarnos con el pgAdmin
GRANT CONNECT ON DATABASE "mi-db" TO egsmartin;
```

veamos más ejemplos:

```psql
-- Solo lectura (SELECT) en todas las tablas existentes:
GRANT SELECT ON ALL TABLES IN SCHEMA public TO egsmartin;

-- O lectura, inserción, actualización y borrado:
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO egsmartin;

-- Permisos sobre las secuencias (necesario para campos AUTOINCREMENT / SERIAL):
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO egsmartin;
```

dar todos los permisos para las tablas y las secuencias dentro del esquema `public`

```psql
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO egsmartin;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO egsmartin;
```

permisos automáticos para futuras tablas:

```psql
ALTER DEFAULT PRIVILEGES IN SCHEMA public 
GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO egsmartin;
```

cambiar el owner de la base de datos:

```psql
ALTER DATABASE "mi-db" OWNER TO egsmartin;
```

podemos ver información de una tabla

```psql
\z personas
```