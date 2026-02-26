Vamos a crear un cluster de Postgres con dos particiones, la primaria y una replica. El almacenamiento lo gestionaremos usando PVC que harán referencia a una clase de almancenamiento - que también crearemos - para que proporcione el almacenamiento en el host.

## Clase de almacenamiento dinámico

Vamos a usar PVC que utlizarán almacenamiento dinámico (es decir, que usarán una clase de almancenamiento en lugar de un PV). Creamos la clase de almacenamiento que permite a nuestro cluster de kind usar el almancemiento local:

```ps
kubectl apply -f https://raw.githubusercontent.com/rancher/local-path-provisioner/master/deploy/local-path-storage.yaml
```

podemos comprobar que efectivamente se ha creado la clase de almancenamiento:

```ps
kubectl get storageclass

NAME                 PROVISIONER             RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION   AGE
hostpath             rancher.io/local-path   Delete          WaitForFirstConsumer   false                  4m49s
local-path           rancher.io/local-path   Delete          WaitForFirstConsumer   false                  7s
standard (default)   rancher.io/local-path   Delete          WaitForFirstConsumer   false                  5m7s
```

se ha creado `local-path` (las otras dos clases se han creado por defecto).

## Usando CloudNativePG 

Con CloudNativePG lo que creamos es un operador que introduce un objeto custome en nuestro cluster kubernetes, el objeto `Cluster` dentro de la api `postgresql.cnpg.io/v1`. Una vez instalado el operador creamos el recurso Cluster. En [cnpg-cluster.yaml](cnpg-cluster.yaml) tenemos los recursos que vamos a crear:

- namespace. todos los objetos los vamos a crear en un nuevo namespace, para gestionarlos mejor
- secret. las credenciales que usaremos con postgress las declaramos en un secrets
- el propio recurso custom `Cluster`
 
El script de instalación de postgress hace lo siguiente:
- Añade el chart de CloudNativePG y lo instala en el namespace `cloudnative-pg`. Recordemos que el chart de helm se guarda en el cluster
- Se asegura de que exista la StorageClass `local-path`. El almacenamiento que necesitamos para Postgress lo proporcionamos con una PVC que usa almacenamiento dinámio. Esta clase es la que usaremos para Postgress. Esta clase funciona con el cluster kubernetes kind para proporcionar almacenamiento local en mi pc
- Crea los objetos kubernetes declarados en [cnpg-cluster.yaml](cnpg-cluster.yaml).
- Espera hasta que el CR `Cluster/my-postgres` esté en condición `Ready`.

Al crear el chart de Help hemos indicado una serie de [parametros](cnpg-cluster.yaml): 

- `instances: 2` (1 primaria + 1 réplica)
- `postgresql.version: "16"`
- `storage.size: 2Gi` y `storageClassName: local-path`
- Usa el Secret `mi-db-secret` y la clave `postgresql-password` para la contraseña del usuario `egsmartin`.

## Uso de Postgres

