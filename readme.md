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
docker build -t kuard .
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

**NOTA**: tambien podemos hacer un port forward a un servicio. Si hubieramos hecho `kubectl expose kuard`, podríamos hacer un port-forward al servicio con `kubectl port-forward svc\kuard 9000:8080`

Cuando tenemos Pods creados con deployments el nombre del pod se asigna automáticamente con un guid, pero podemos gestionar esto de forma automática, por ejemplo como sigue:

```ps
$MIPOD=kubectl get pods -l app=alpaca-prod -o jsonpath='{.items[0].metadata.name}'
kubectl port-forward $MIPOD 9000:8080
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

Con Service Discovery se resuelve la direccion de cada proceso. Típicamente se usa DNS para resolver este problema, pero en casos en los que un proceso cambia o puede cambiar de dirección con cierta frecuencia, el DNS no es un mecanismos adecuado, incluso cuando bajamos el _TTL_ para asegurar que los clientes _repitan_ la resolución de direcciones con cierta frecuencia. En un entorno como Kubernetes la frecuencia con la que la direccion de los procesos cambia (porque se destruye el proceso, se reubica, o se incrementa el número de instancias del servicio).

Para exponer un servicio se usa `kubectl expose`. Para entenderlo mejor vamos a crear un _deployment_

```ps
kubectl create deployment alpaca-prod --image=docker.io/egsmartin/kuard:latest --port=8080
```

aumentamos el número de replicas:

```ps
kubectl scale deployment alpaca-prod --replicas 3
```

exponemos el deployment como un servicio:

```ps
kubectl expose deployment alpaca-prod
```

Con este comando __Kubernetes intentará crear un Service, pero como no has especificado los puertos, fallará o usará valores por defecto (usara el mismo puero que hayamos elegido exponer en el pod o en el deployment)__ que probablemente no funcionen como esperamos. Al menos debemos indicar:

- __port__: el puerto por el que se accederá al servicio.
- __target-port__: el puerto interno del contenedor al que se redirige el tráfico (si es diferente).
- __type__: opcional, para definir si el servicio es `ClusterIP`, `NodePort`, `LoadBalancer`, etc.

__Si queremos exponer más de un puerto, con expose no se puede hacer, se necesitaria usar el objeto `Service` directamente__. Si exponemos directamente el servicio con `Service` directamente, en la especificacion __podemos incluir varias tuplas port `targetPort` name__, En caso de que el tipo de servicio fuerta `NodePort` se crearán automáticamente tantos puertos en los nodos como tuplas tengamos.

Recordar que cuando llamamos al servicio con el LoadBalancer los puertos que usaremos seran los mismos que si llamasemos al servicio usando la IP que le asigno Kubernetes para el cluster.

Creamos otro deployment, aumentamos las replicas y exponemos el servicio:

```ps
kubectl create deployment bandicoot-prod --image=docker.io/egsmartin/kuard:latest --port=8080

kubectl scale deployment bandicoot-prod --replicas 2

kubectl expose deployment bandicoot-prod
```

podemos ver los servicios que acabamos de crear:

```ps
kubectl get services -o wide

NAME             TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)    AGE   SELECTOR
alpaca-prod      ClusterIP   10.96.107.253   <none>        8080/TCP   19m   app=alpaca-prod
bandicoot-prod   ClusterIP   10.96.37.174    <none>        8080/TCP   18m   app=bandicoot-prod
kubernetes       ClusterIP   10.96.0.1       <none>        443/TCP    63m   <none>
```

Kubernetes asigna a cada servicio una `Cluster IP`, que es una direccion virtual que representa el servicio. Si hay varias réplicas en el deployment al que apunta el servicio, cuando llamemos al servicio la petición puede dirigirse a cualquiera de esas réplicas. Tenemos tres tipos de servicio:

- `ClusterIP` (defecto)
- `NodePort`
- `LoadBalancer`

NodePort y LoadBalancer incluyen la funcionalidad de ClusterIP. Cuando defines un Service como LoadBalancer, Kubernetes:

- Crea un ClusterIP para comunicación interna.
- Crea un NodePort automáticamente (aunque no lo veas explícitamente).
- Solicita al proveedor cloud que cree un balanceador de carga externo que apunte al NodePort.

Cuando usas un Service de tipo LoadBalancer en Kubernetes, se crea un artefacto externo al clúster, que es un balanceador de carga proporcionado por el proveedor de infraestructura (como AWS ELB, Azure Load Balancer, GCP Load Balancer, etc.). 

Kubernetes crea un Service de tipo LoadBalancer, el proveedor cloud detecta esta solicitud y crea un balanceador de carga externo. Ese balanceador tiene como backends:

- Las IP de los nodos del clúster.
- Los puertos NodePort que Kubernetes asigna automáticamente.

El tráfico que llega al balanceador se enruta a los nodos, y de ahí al Service, que lo redirige a los Pods correspondientes.

### DNS

Kubernetes proporciona un servicio de DNS en cada nodo del cluster. Cuando creamos un servicio automáticamente se crea un _A record_ en el DNS:

`alpaca-prod.default.svc.cluster.local.`

tenemos el nombre del servicio, seguido del namespace, la etiqueta `svc`, y el dominio del cluster. Podemos referirnos desde cualquier pod a este servicio y el DNS resolverá este nombre a la `Cluster IP`. Si estamos en el mismo namespace que el servicio podremos referirnos al servicio simplemente como `alpaca-prod`.

En la aplicación `kuard` tenemos una opción para poder consultar el DNS.

El `Service` reconoce el readiness probe y de modo que solo se enviará tráfico a un determinado Pod cuando es listo. Podemos ver los endpoints de un servicio:

```ps
kubectl get endpoints alpaca-prod --watch

NAME          ENDPOINTS                                         AGE
alpaca-prod   10.244.1.3:8080,10.244.1.5:8080,10.244.1.6:8080   27m
```

las IPs que se muestran son las IPs de los Pods. Figuran tres porque el servicio está apuntando a un deployment que tiene tres IPs. Si hacemos con kuard que un pod falle su readiness probe, será sustituido por otro, que tendrá una IP diferente.

Nótese que asociado a cada servicio se crea un objeto `Endpoint` que dinámicamente se ajusta a las direcciones de los Pods que están detrás del servicio.

### Service discovery manual

Lo que viene a hacer Kubernetes para hacer el discovery es lo siguiente. En primer lugar identifica cuales son los Pods que se corresponden con un servicio/deployment, utilizando la etiqueta `app` (los pods que se crean con un deployment se etiquetan añadiendo con la key `app` y el valor con el nombre del deployment). Las IPs de los Pod serán las IPs de los endpoints del servicio:

```ps
kubectl get pods -o wide --show-labels

kubectl get pods -o wide --selector=app=alpaca-prod
```

### Kube-proxy y Cluster IPs

Las IP Virtuales funcionan porque en cada nodo tenemos un proxy - `kube-proxy` - que intercepta todas las peticiones y las resuelve, en el caso de una IP Virtual, a uno de los endpoints asociados al servicio.

![Proxy](./imagenes/proxy.png)

El kube-proxy monitoriza el API server para detectar cuando se están creando nuevos servicios. Cuando se crea un nuevo servicio el Kube-proxy actualiza las reglas definidas en las `iptables` que gestiona el `kernel` de cada nodo, de modo que se re-escriban los destinos de los paquetes generados en el nodo y se redirijan a los endpoints del servicio. Cuando los endpoints de un servicio cambian (Pods que se añaden, Pods que se retiran, por ejemplo, por no superar el readiness check), las reglas en las iptables se actualizan. La cluster IP como que asigna el API server cuando se crea el servicio no cambia (habría que borrar y recrear el servicio).

El rango de IPs que se asignan a los servicios se configuran con el parámetro `--service-cluster-ip-range` en el binario del `kube-apiserver`. Este rango de IP no se tiene que solapar con las subredes y rangos asignados a los Pods/Nodos.

### Variables de Entorno

Además de actualizar el DNS y el Kube-proxy, despues que se crea un servicio, cuando se cree un Pod Kubernetes definirá una serie de variables de entorno para cada servicio creado (los Pods que ya estaban creado no tendrán estas variables de entorno). Por ejemplo para el servicio `alpaca-prod` que hemos creado, si lanzamos un Pod veremos en él las siguientes variables de entorno:

```ps
ALPACA_PROD_SERVICE_HOST	10.96.107.253
ALPACA_PROD_SERVICE_PORT	8080
ALPACA_PROD_PORT_8080_TCP_PROTO	tcp
ALPACA_PROD_PORT_8080_TCP_ADDR	10.96.107.253
ALPACA_PROD_PORT_8080_TCP_PORT	8080
ALPACA_PROD_PORT	tcp://10.96.107.253:8080
ALPACA_PROD_PORT_8080_TCP	tcp://10.96.107.253:8080
```

### Limpiamos

```ps
kubectl delete services,deployments -l app
```

## Ingress

El controlador de Ingress esta compuesto por dos partes. 

- __Proxy de Ingress__: se expone fuera del clúster utilizando un servicio de tipo: LoadBalancer. Este proxy envía solicitudes a servidores "upstream"

- __Reconciliador de Ingress u operador__. El operador de Ingress es responsable de leer y monitorizar objetos de Ingress en la API de Kubernetes y reconfigurar el proxy de Ingress para enrutar el tráfico como se especifica en el recurso de Ingress

![Ingress](./imagenes/ingress.png)

No existe un controlador de Ingress "estándar" integrado en Kubernetes, por lo que el usuario debe instalar uno de entre las muchas implementaciones disponibles. Vamos a utilizar __Contour__.

Para instalarlo:

```ps
kubectl apply -f https://projectcontour.io/quickstart/contour.yaml
```

comprobamos 

```ps
kubectl get svc envoy -n projectcontour -o wide
```

Los recursos se crean en el namespace `projectcontour`. Podemos ver un deployment (con dos replicas) y un servicio de tipo `LoadBalancer`

```ps
kubectl get svc,deployments,pods -n projectcontour

NAME              TYPE           CLUSTER-IP      EXTERNAL-IP   PORT(S)                      AGE
service/contour   ClusterIP      10.96.222.28    <none>        8001/TCP                     3m41s
service/envoy     LoadBalancer   10.96.177.192   172.18.0.6    80:30796/TCP,443:32096/TCP   3m41s

NAME                      READY   UP-TO-DATE   AVAILABLE   AGE
deployment.apps/contour   2/2     2            2           3m41s

NAME                                READY   STATUS      RESTARTS   AGE
pod/contour-5cb8d455bc-pnrt5        1/1     Running     0          3m41s
pod/contour-5cb8d455bc-v5f9g        1/1     Running     0          3m41s
pod/contour-certgen-v1-33-1-dgqkw   0/1     Completed   0          3m41s
pod/envoy-2fj22                     2/2     Running     0          3m41s
```

Se crea tambien una `CustomResourceDefinition`. 

Al tratarse de una instalación global en el cluster, para poder crear este recurso tenemos que tener permisos de administrador para el cluster.

Para que Ingress funcione correctamente es necesario actualizar el DNS incluyendo la dirección externa del balanceador de carga. Si por ejemplo nuestro dominio fuera `example.com`. Tendremos que configurar dos entradas en el DNS: alpaca.example.com y bandicoot.example.com. 

Si tienes una dirección IP para tu balanceador de carga externo, querrás crear registros A (esto es, asignar la `EXTERNAL-IP` el dominio del balanceador - A record -, y crear alias que apunten al balanceador - CNAME records). Con esto las peticiones al balanceador, o a los alias se enrutaran hacia ingress (en local podemos actualizar el archivo _hosts_).

```ps
kubectl apply -f ejemplos.yaml

kubectl get deployment -o wide

kubectl get services -o wide
```

### Ingress

Al hacer `ejemplos.yaml` hemos creado tambien un objeto Ingress:

```ps
kubectl get ingress -o wide
```

la definición de Ingress es la siguiente:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: simple-ingress
  annotations:
    kubernetes.io/ingress.class: contour
  defaultBackend:
    service:
      name: multiplica
      port:
        number: 8080    
spec:

[...]
```

que define varias reglas. Cada regla se activa cuando la petición se recibe de un determinado `host`. En la regla especificamos diferentes backends en función del `path` utilizado. Para indicar el backend se informa el nombre del servicio y el puerto que hay que utilizar. Por ejemplo para `gz.com` definimos do rutas:

```yaml
  rules:
    - host: gz.com
      http:
        paths:
          - path: /multiplica
            pathType: Prefix
            backend:
              service:
                name: multiplica
                port:
                  number: 8080
          - path: /primos
            pathType: Prefix
            backend:
              service:
                name: primos
                port:
                  number: 8080

[...]
```

nótese que hemos definido un backend por defecto, así que cuando ninguna de las reglas cualifica (por ejemplo si llamamos a localhost - [recordemos que localhost esta mapeado por el puerto 80 y 443 con el Pod de Contour/Envoy](networking.md))

Veamos los recursos ingres que tenemos creados:

```ps
kubectl get ingress


NAME             CLASS     HOSTS                                    ADDRESS      PORTS   AGE
simple-ingress   contour   gz.com,multiplica.gz.com,primos.gz.com   172.18.0.6   80      9m57s
```

Es importante destacar que con el Ingress que hemos creado arriba cuando llamamos a `gz.com/multiplica` se enviará la petición al backend `multiplica` puerto 8080, pero **importante, el recurso que llegará será `/multiplica`**. Esto significa que **no se hace un rewrite**. Para hacer un rewrite tenemos que usar otro objeto `HTTPProxy`.

### HTTPProxy

El objeto Ingress se incorporo en Kubernetes en la versión 1.1 para describir un reverse proxy de forma global dentro de un cluster. Desde entoces el objeto Ingress no ha evolucionado lo que ha hecho que profileferen anotaciones por medio de las cueles extender la funcionalidad de enrutado de Ingress (que es muy limitada). Con el objeto **HTTPProxy** Contour proporciona una *Custom Resource Definition (CRD)* que ![evoluciona la funcionalidad de Ingress](https://projectcontour.io/docs/v1.4.0/httpproxy/).

