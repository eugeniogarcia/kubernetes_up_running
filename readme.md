## dockerfile

Antes de comentar el dockerfile, refrescar algunos comandos de go que pueden ser interesantes:

```ps
go install 
```

compila el programa y lo _instala_ en carpeta donde se guardan los ejecutables de go (`go build` tambien hace la compilación, pero el ejecutable se guarda en el propio directorio desde el que se ejecuta el _build_). La carpeta en la que se guarda el binario se determina con la variable de entorno `GOBIN`. Si esta variable no existiera, entonces el ejecutable se guarda en `$GOPATH/bin`, y sino tuvieramos variable _GOPATH_ se guardará en `$HOME/go/bin`.

Con `go vet` comprobamos si hay algun problema, o algun paquete deprecado. Con `go tidy` lo que se hace es revisar todos los paquetes que se usan y asegurar que esten presentes en `go.mod` y al tiempo se eliminan aquellas referencias que no se usen. Típicamente ejecutamos `go tidy` cuando actualizamos los paquetes que usamos.

Para construir la utilidad `kuard` usaremos un dockerfile multistage. Comentar algunas cosillas:

- Usamos como imagen base Alpine, que es una imagen mínima que ya contiene _golang_. Esta ditribucuión no incluye _apt_, es necesario usar _apk_. Con _apk_ tenemos acceso a menos paquetes de terceros, lo que puede ser una limitación para usar _alpine_. Esta primera imagen la usamos para construir la aplicación (nótese el `AS build`)

```dockerfile
FROM golang:alpine AS build
```

- Al hacer `RUN en una sola linea, creamos una sola capa en la imagen, en lugar de crear dos. En este script se realiza la compilación e instalación de _kuard_

```dockerfile
RUN dos2unix build/build.sh && \
    VERBOSE=0 PKG=kuard ARCH=amd64 VERSION=test bash build/build.sh
```

- Se copia el resultado de la compilación a partir de la imagen `build`. Nos aseguramos que el ejecutable no tenga permisos con el `chown`

```dockerfile
COPY --from=build --chown=nobody:nobody /go/bin/kuard /kuard
```

- se instala el paquete  `ca-certificates` npm (e indirectamente nodejs para instalar npm), pero lo hacemos ya en la imagen final

```dockerfile
RUN apk add --no-cache ca-certificates
```

- Ejecutamos con un usuario sin privilegios

```dockerfile
USER nobody:nobody
```

- Cuando la imagen se ejecuta en docker podemos crear un health check que es _análogo_ a los probes que podemos definir en un Pod de Kubernetes:

```dockerfile
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/kuard", "-v"] || exit 1
```

### Construcción

Construimos la imagen:

```ps
kuard> docker build -t kuard .
```

podemos ver las imagenes:

```ps
docker images
```

vamos a taggearla y a publicarla en Docker Hub:

```ps
docker login

docker tag kuard:latest egsmartin/kuarg:latest

docker push egsmartin/kuarg:latest
```

podemos ejecutar en docker:

```ps
docker run --rm -p 8080:8080 egsmartin/kuard:latest
```

```ps
docker run -d --name kuard \
  --publish 8080:8080 \
  --memory 200m \
  --memory-swap 1G \
  --cpu-shares 1024 \
  egsmartin/kuard:latest
```

finalmente liberamos recursos que ya no necesitamos:

```ps
docker system prune
```

## kubectl

```ps
kubectl version
```

### Estado del cluster

- Estado del control plane (API server, scheduler, controller-manager). Esto te da un diagnóstico real del API server y sus dependencias:

```ps
kubectl get --raw='/readyz?verbose'
```

- Estado de los nodos

```ps
kubectl get nodes
```

podemos a continuación ver la información detallada de un nodo:

```ps
kubectl describe node desktop-control-plane
```
Veremos: nombre, `Labels`, `Annotations`, `Taints`, `unschedulable` (true|false), `Conditions` (lista de mensajes con estados del nodo y su timestamp), IP, `Capacity` (cpu, almacenamiento, memoria, número de pods), `Allocatable` (que recursos estan libres para ser usados), `System Info` (información del sistema), `Memory Limits` de los pods asignados al nodo, `Allocated resources` (muestra los recursos asignados como `requests` y como `limits`; Es un agregado de los valores de los pods asignados al nodo), `Events`

- Estados de los pods del plano de control:

```ps
kubectl get pods -n kube-system

NAME                                            READY   STATUS    RESTARTS   AGE
coredns-7d764666f9-bqjm6                        1/1     Running   0          16m
coredns-7d764666f9-rjbtx                        1/1     Running   0          16m
etcd-desktop-control-plane                      1/1     Running   0          16m
kindnet-rft97                                   1/1     Running   0          16m
kindnet-ttnnw                                   1/1     Running   0          16m
kube-apiserver-desktop-control-plane            1/1     Running   0          16m
kube-controller-manager-desktop-control-plane   1/1     Running   0          16m
kube-proxy-4k9tv                                1/1     Running   0          16m
kube-proxy-qmxvq                                1/1     Running   0          16m
kube-scheduler-desktop-control-plane            1/1     Running   0          16m
```

podemos ver los siguientes elementos:
    - `dns`. En cada nodo corre un pod que proporciona los servcios DNS del cluster
    - `proxy`. En cada nodo corre un pod que proporciona un proxy que intercepta todas las comunicaciones del pod. Por ejemplo cuando accedemos a un servicio el proxy contiene información del los _endpoints_ de ese servicio y distribuye las llamadas hacia ellos
    - `apiserver`. Expones las apis de kubernetes
    - `etcd`. Capa en la que se almacena el estado
    - `controller`. Controlador de kubernetes que identifica los recursos que deben crearse
    - `scheduler`. Se encarga de identificar y solicitad de gestionar los recursos en los nodos del cluster

```ps
kubectl exec -n kube-system etcd-desktop-control-plane -- etcdctl endpoint health
```

Muchos de estos comandos admiten el flag `--watch` para monitorizar de forma continua cambios de valor/estado.

### Namespaces

Con kubectl hacemos referencia a un namespace con el tag `--namespace` o el abreviado `-n`.

Podemos fijar el namespace por defecto usando un contexto. Creamos un contexto:

```ps
kubectl config set-context my-context --namespace=mystuff
```

y lo usamos:

```ps
kubectl config use-context my-context
```

### Gestion de recursos

Podemos obtner información de los recursos creados. Por ejemplo para recuperar los pods y servicios:

```ps
kubectl get pods,services
```

podemos ver que opciones nos da la api de kubernetes con cada recurso. Por ejemplo, los atributos disponibles en los pods:

```ps
kubectl explain pods
```

por defecto se muestra el primer nivel de propiedades. Si queremos recuperar todas las propiedades:

```ps
kubectl explain pod --recursive=true
```

creamos y borramos recursos:

```ps
kubectl apply -f kuard-pod-full.yaml

kubectl delete -f kuard-pod-full.yaml
```

podemos también referirnos a los objetos por su nombre:

```ps
kubectl delete <resource-name> <obj-name>
```

podemos etiquetar los objetos:

```ps
kubectl label pods bar color=red
```

si queremos quitar la etiqueta usamos el `-`. Por ejemplo para eliminar la etiqueta `color` del pod `bar` haríamos:

```ps
kubectl label pods bar color-
```

### Depurar contenedores

Para ver los logs de un pod, por ejemplo del pod `kuard`:

```ps
kubectl logs kuard
```

podemos ver todos los eventos con:

```ps
kubectl describe pod kuard
``` 

nos podemos conectar en el contenedor ejecutando bash (siempre y cuando este presente en la imagen, que NO es el caso de _alpine_):

```ps
kubectl exec -it kuard -- bash
```

en el caso de _alpine_ tenemos el shell `ash`:

```ps
kubectl exec -it kuard -- ash
```

podemos ejecutar cualquier comando, no solo el shell:

```ps
kubectl exec -it kuard -- date
```

Si no tenemos bash disponible en la imagen nos podemos _enganchar_ a ella, y veremos la salida por consola - que veríamos si estuvieramos corriendo el programa en forma local:

```ps
kubectl attach -it kuard
```

podemos copiar archivos - subirlos - a un pod:

```ps
kubectl cp <pod-name>:</path/to/remote/file> </path/to/local/file>
```

Otra opción __muy interesante es el port forwarding__. Por ejemplo para que nuestro _localhost:9000_ se mapee con el puerto 8080 del pod:

```ps
kubectl port-forward kuard 9000:8080
```

podemos ver los eventos del cluster:

```ps
kubectl get events
```

si queremos ver que recursos consumen más recursos (siempre y cuando el cluster tenga habilitadas las métricas):

```ps
kubectl top nodes

kubectl top pods
```

### Gestión del Cluster

Si queremos que el scheduler deje de programar recursos en un determinado nodo:

```ps
kubectl cordon desktop-worker
```

Para extraer los recursos que ya están corriendo en un nodo:

```ps
kubectl drain desktop-worker
```

para volver a permitir que un nodo que fue acordonado vuelva a recibir encargos por parte del scheduler:

```ps
kubectl uncordon desktop-worker
```

## Pods

Vamos a describir las características principales de un Pod. Vamos a tratar el pod kuard. Lo creamos:

```ps
kubectl apply -f .\kuard-pod-full.yaml
```

podemos ver información de detalle

```ps
kubectl describe pods kuard
```

podemos hacer port forwarding al contenedor:

```ps
kubectl port-forward kuard 8080:8080
```

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: kuard
spec:
  volumes: # podemos definir volumenes que estarán disponibles para todos los contenedores del pod
    - name: "kuard-data" # nombre del volumen
      emptyDir: {} # en este caso vamos  crear un volumen temporal en memoria. Podriamos haber creato un ntfs, hostPath, persistentVolumeClaim, etc.
  containers:
    - image: docker.io/egsmartin/kuard:latest # tomamos una imagen en este caso de docker hub
      imagePullPolicy: IfNotPresent # política de pull de la imagen
      name: kuard
      ports:
        - containerPort: 8080 # puerto que expone el contenedor. Lo llamamos http
          name: http
          protocol: TCP
      resources:
        requests: # recursos que solicita el contenedor. El scheduler usará esta información para asignar el pod a un nodo que tenga estos recursos disponibles. El pod podria usar más cantidad de recursos si estuvieran disponibles en el nodo, pero nunca menos de lo solicitado, ni más de lo permitido por los límites
          cpu: "500m" # 0.5 cores
          memory: "128Mi" # 128 megabytes (las i significa que utilizamos potencias de 2, es decir 1Mi = 1024 * 1024 bytes)
        limits: # recursos máximos que puede usar el contenedor. Si llegara a superar este límite, el contenedor sería terminado
          cpu: "1000m"
          memory: "256Mi"
      volumeMounts: # vamos a montar en esta imagen uno de los volumnes que hemos definido en el pod, el que tiene nombre kuard-data. Lo montamos en la ruta /data del contenedor
        - mountPath: "/data"
          name: "kuard-data"
      livenessProbe: # esta probe se usa para comprobar si el contenedor está vivo. Si falla, el contenedor se reiniciará. Se hace una llamada a la ruta /healthy del contenedor en el puerto 8080, cada 10 segundos, con un timeout de 1 segundo. Si falla 3 veces consecutivas, se considera que el contenedor no está vivo y se reinicia. La primera comprobación se hace 5 segundos después de iniciar el contenedor
        httpGet:
          path: /healthy
          port: 8080
        initialDelaySeconds: 5
        timeoutSeconds: 1
        periodSeconds: 10
        failureThreshold: 3
      readinessProbe: # esta probe se usa para comprobar si el contenedor está listo para recibir tráfico. Si falla, el contenedor no recibirá tráfico (pero no se reinicia). Se hace una llamada a la ruta /ready del contenedor en el puerto 8080, cada 10 segundos, con un timeout de 1 segundo. Si falla 3 veces consecutivas, se considera que el contenedor no está listo y no recibirá tráfico. La primera comprobación se hace 30 segundos después de iniciar el contenedor
        httpGet:
          path: /ready
          port: 8080
        initialDelaySeconds: 30
        timeoutSeconds: 1
        periodSeconds: 10
        failureThreshold: 3
```

## Etiquetas

Las etiquetas son una forma de metadata que puede asociarse a diferentes objetos y que puede utilizarse para seleccionar objetos. Las etiquetas son un par key valor. La key debe ser un valor _valido DNS_. Podemos ver varios ejemplos de etiquetas:

|Key|Value|
|-----|-----|
|acme.com/app-version|1.0.0|
|appVersion|1.0.0|
|app.version|1.0.0|
|kubernetes.io/cluster-service|true|

Al recuperar los objetos podemos incluir etiquetas en la salida (`-L` es un shortcut de `--show-labels`):

```ps
kubectl get deployments -L canary
```

para actualizar el valor de una etiqueta usamos el comando `label`:

```ps
kubectl label deployments alpaca-test "canary=true"
```

para eliminar una etiqueta ponemos el sufijo `-` en el nombre de la etiqueta:

```ps
kubectl label deployments alpaca-test canary-
```

Podemos usar las etiquetas para _interrogar_ a Kubernetes a la hora de recuperar objetos. Por ejemplo aquí vamos a obtener deployments con la etiqueta _canary_

```ps
kubectl get pods --selector="ver=2"
```

si indicamos dos selectores separados por una coma se interpreta como un AND:

```ps
kubectl get pods --selector="ver=2,app=bandicoot"
```

podemos usar varios operadores:

|Operator|Description|
|--------|--------|
|key=value|key is set to value|
|key!=value|key is not set to value|
|key in (value1, value2)|key is one of value1 or value2|
|key notin (value1, value2)|key is not one of value1 or value2|
|key|key is set|
|!key|key is not set|

## Anotaciones

Son tambien metadata pero una metadata que no se utiliza para clasificar sino para registrar información con el contenedor que no tiene cabida en la api estandar de Kubernetes.

## Service Discovery


