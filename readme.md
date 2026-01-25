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

kubectl get pods my-pod -o jsonpath --template={.status.podIP}

kubectl get pods,services

kubectl explain pods

kubectl apply -f obj.yaml

kubectl delete -f obj.yaml

kubectl delete <resource-name> <obj-name>

### Operar con Pods

kubectl label pods bar color=red

kubectl label pods bar color-

kubectl logs <pod-name>

kubectl exec -it <pod-name> -- bash

kubectl attach -it <pod-name>

kubectl cp <pod-name>:</path/to/remote/file> </path/to/local/file>

kubectl port-forward <pod-name> 8080:80

kubectl get events

uso de recursos

kubectl top nodes

kubectl top pods

### Gestión del Cluster

kubectl cordon

kubectl drain

kubectl uncordon


