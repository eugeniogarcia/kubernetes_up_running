## Creación del Operador

El operador se encarga de controlar los recursos custom que creamos en el cluster. El operador propiamente dicho se despliega con un deployment. Contactaremos usando la api-server para ver que recursos de nuestro custom respurce se han creado, y asegurar que se traslade ese estado al cluster, creando, modificando y eliminando lo que corresponda. Esto es lo que se denomina loop de reconciliación. 

Para identificar nuestro recurso utilizamos el grupo, version y nombre:

```go
var (
	gvr = schema.GroupVersionResource{Group: "gz.com", Version: "v1", Resource: "multiplicadores"}
)
```

configuración del cliente (client go) que hace las peticiones al api server de kubernetes. Con este cliente se interactua con el api-server de kubernetes para gestionar los recursos del cluster (crear pods, deployments, borrar,...), lo que sea necesario.

Soporta dos modos: **desarrollo local** (toma el principal del kubeconfig) y **ejecución dentro del cluster** (el principal es la service account del pod en el que corre el operador); El principal tiene que tener los premisos (esto es, tene que haber un _rolebinding_ que asocie el principal con un _role_ que incluya los permisos necesarios para gestionar el propio recurso a medida y los recursos asociados con él.

```go
func buildConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	return rest.InClusterConfig()
}
```

para interactuar con los recursos de kubernetes emplearemos esta configuración, pero empleando dos interfaces diferentes:

- **dyn (dynamic.Interface)**: Cliente dinámico para interactuar con recursos personalizados (CRs). Permite trabajar **con cualquier recurso sin necesidad de tener tipos Go predefinidos**. Es flexible pero menos type-safe. Se usa cuando trabajas con Custom Resource Definitions (CRDs) o recursos dinámicos cuya estructura no está predefinida en el código.

- **clientset (kubernetes.Clientset)**: Cliente tipado para **interactuar con recursos core de Kubernetes**
como Pods, Deployments, Services, ConfigMaps, etc. Proporciona métodos específicos y type-safe para cada tipo de recurso. Es más eficiente y ofrece mejor autocompletado IDE.

```go
// crea un cliente dinámico para interactuar con recursos personalizados (CRs)
dyn, err := dynamic.NewForConfig(cfg)
if err != nil {
    log.Fatalf("No se pudo crear el cliente dinámico: %v", err)
}

// crea un clientset tipado para interactuar con recursos core como Deployments y Services
clientset, err := kubernetes.NewForConfig(cfg)
if err != nil {
    log.Fatalf("No se pudo crear el clientset: %v", err)
}
```

el loop de control lo implemetamos con dos canales, uno que se despierta con una frecuencia - a la que comprobaremos si hay cambios en la configuración -, y otro que se despierta con una interrupción:

```go
// creamos un contexto que permita la cancelación con SIGINT/SIGTERM para que el operador pueda apagarse de manera ordenada.
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
// aseguramos que el contexto se cancele y los recursos se limpien al salir
defer stop()
// un ticker para ejecutar reconcileAll cada 5 segundos
ticker := time.NewTicker(5 * time.Second)
// aseguramos que el ticker se detenga al salir para liberar recursos
defer ticker.Stop()
```

y el loop quedaría:

```go
	// loop infinito de control que implementa el controlador
	for {
		if err := reconcileAll(ctx, dyn, clientset); err != nil {
			log.Printf("error de reconciliación: %v", err)
		}

		select {
		case <-ctx.Done(): //se ha abortado la ejecución
			log.Println("apagando el operador multiplicador")
			return
		case <-ticker.C: //tick del reloj
		}
	}
```

cada cinco segundos recuperamos todos los recursos _Multiplicador_ con el api-server:

## Reconciliación

```go
func reconcileAll(ctx context.Context, dyn dynamic.Interface, clientset kubernetes.Interface) error {
	// Lista todos los recursos del namespace, correspondientes al CustomResource que controlamos. Los recursos se identifican de forma univoca con el grupo de api, versión y nombre. Se trata de un recurso custom así que usamos el cliente dinámico
	list, err := dyn.Resource(gvr).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("lista multiplicadores: %w", err)
	}
	// recorremos cada uno de los recursos que hemos identificado...
	for _, item := range list.Items {
		// hace la reconciliación de ese recurso concreto
		if err := reconcileOne(ctx, dyn, clientset, &item); err != nil {
			log.Printf("reconciliar %s/%s falló: %v", item.GetNamespace(), item.GetName(), err)
		}
	}
	return nil
}
```

y para cada uno de ellos aplica la lógica de reconciliación:
- Asegurar que los recursos asociados esten creados
- Comprobar que tengan las propiedades esperadas

### Asegurar que los recursos estándard esten creado

En primer luegar leemos las propiedades que tiene el objeto:

```go
// recupera la propiedad replicas de nuestro custom resource. Esta propiedad se debe mapear al número de replicas del deployment. Sino se especifica se toma 1 por defecto:
replicas64, ok := getIntField(u, "spec", "replicas")
var replicas int32 = 1
if ok {
    replicas = int32(replicas64)
}

// obtiene la propiedad multiplicador del custom resource. Esta propiedad se debe mapear a la variable de entorno MULTIPLIER del deployment. Sino se especifica se toma 2 por defecto:
multiplicador64, ok := getIntField(u, "spec", "multiplicador")
var multiplicador int64 = 2
if ok {
    multiplicador = multiplicador64
}
// obtiene las propiedades ruta y host del recurso a medida
ruta, _ := getStringField(u, "spec", "ruta")
host, _ := getStringField(u, "spec", "host")
```

comprobamos que el deployment asociado a nuestro custom resource este creado, o lo creamos:

```go
existing, err := clientset.AppsV1().Deployments(ns).Get(ctx, deplName, metav1.GetOptions{})
	if err != nil {
		// creamos un deployment
		d := makeDeployment(deplName, ns, replicas, multiplicador)

[...]
```

caso de que tengamos que crear el deployment vamos a:
- crear el deployment
- relacionar el deployment con el custom resource (de modo que kubernetes sepa que están relacionados y se elimine el deployment cuando se elimine el custom resource)

```go
gvk := u.GroupVersionKind()
if gvk.Empty() {
    gvk = schema.GroupVersionKind{Group: gvr.Group, Version: gvr.Version, Kind: "Multiplicador"}
}
ownerRef := metav1.NewControllerRef(u, gvk)
d.ObjectMeta.OwnerReferences = append(d.ObjectMeta.OwnerReferences, *ownerRef)
_, err := clientset.AppsV1().Deployments(ns).Create(ctx, d, metav1.CreateOptions{})
```

- creamos el httpproxy y el servicio

```go
if err := ensureServiceAndProxy(ctx, dyn, clientset, u, deplName, ruta, host); err != nil {
```

Llegados a este punto tenemos creado el deployment, service y httpproxy relacionado con el recurso a medida que estamos reconociliando

### Verificar que el estado de cada recurso sea el correcto

Comprobamos que el número de replicas definido y el real sean las mismas, y en caso contrario tendremos que actualizarlas:

```go
if existing.Spec.Replicas == nil || *existing.Spec.Replicas != replicas {
    existing.Spec.Replicas = &replicas
    needUpdate = true // hay que actualizar el número de replicas
}
```

```go
// actualizamos el deployment si es necesario
if needUpdate {
    if _, err := clientset.AppsV1().Deployments(ns).Update(ctx, existing, metav1.UpdateOptions{}); err != 
```

## Crear Deployment

Para crear lo primero que hacemos es construir la plantilla con todos sus datos:

```go
func makeDeployment(name, namespace string, replicas int32, multiplicador int64) *appsv1.Deployment {

	// etiquetas que vamos a utilizar. Un par de etiquetas
	labels := map[string]string{"app": "multiplicador", "multiplicador-name": name}

	// devolvemos la estructura con todos los datos del Deployment que tenemos que crear
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: labels},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "multiplica",
							Image: "docker.io/egsmartin/multiplica:latest",
							Ports: []corev1.ContainerPort{{ContainerPort: 8080}},
							Env:   []corev1.EnvVar{{Name: "MULTIPLIER", Value: fmt.Sprintf("%d", multiplicador)}},
							// ReadinessProbe omitted for simplicity in this example
						},
					},
				},
			},
		},
	}
}
```

llamamos a este helper para crear una instancia de la clase Deployment:

```go
d := makeDeployment(deplName, ns, replicas, multiplicador)
```

y lo creamos usando el cliente:

```go
_, err := clientset.AppsV1().Deployments(ns).Create(ctx, d, metav1.CreateOptions{})
```

## Crear Servicio y HttpProxy

El Servicio y el HttpProxy se crean con la misma tecnica con la que se creo el Deployment

## Experimento

Creamos todos los recursos con este script:

```ps
.\ejemplos.ps1
```

podemos comprobar que si llamamos a `` obtenemos la respuesta de nuestro servicio.

Si hacemos `kubectl apply -f .\multiplicador-ejemplo3.yaml` se incrementará el número de replicas, y cambiará el multiplicador, pasaremos a multiplicar por cuatro.

Borramos todo con:

```ps
.\borra.ps1
```

## Depura

Para depurar vamos a utilizar delve, el depurador de go. Usaremos Delve para que haga de proxy. Delve expone un puerto remoto por el que nos conectaremos para acceder al operador.

Creamos un `Dockerfile.depura` para crear la imagen con Delve. Los puntos más destacados de este dockerfile son:

- Se instala Delve en la imagen de construcción
- Se compila el programa go con una serie de flags asociadas a Delve (deshabilita optimizaciones del compilador de go, y la creación de funciones inline - por parte del compilador)
- En la imagen final se copia el ejecutable del operador y el ejecutable de Delve
- El entrypoin ya no es el operador, sera Delve
- Delve está escuchando en el puerto 40000

aqui tenemos el dockerfile:

```yaml
#*************************************************************
#*************************************************************
# usamos una imagen base de Go para compilar el operador
#*************************************************************
#*************************************************************
FROM golang:1.25-alpine AS build 
RUN apk add --no-cache git build-base

# creamos el directorio de trabajo y copiamos el código fuente
WORKDIR /src
COPY go.mod go.sum ./
# instalamos las dependencias
RUN go mod download
COPY . .

# instalamos delve para depurar el operador. Instala git y build-base en build (necesario para Delve)
RUN CGO_ENABLED=0 go install github.com/go-delve/delve/cmd/dlv@latest
 
# compilamos -gcflags="all=-N -l":
# -N: Deshabilita optimizaciones, permitiendo que Delve acceda a variables y funciones sin interferencias.
# -l: Deshabilita inlining (expansión de funciones en línea), lo que facilita el stepping durante el debug.
RUN CGO_ENABLED=0 GOOS=linux go build -gcflags "all=-N -l" -o /out/multiplicador-operator ./

#*************************************************************
#*************************************************************
# creamos la imagen final basada en Alpine para ejecutar el operador
#*************************************************************
#*************************************************************
FROM alpine:latest

# instala ca-certificates en runtime (útil para conexiones HTTPS si el operador las usa), y libc6-compat para compatibilidad con binarios compilados en Go (que pueden requerir glibc). Esto es especialmente importante para Delve, que a veces puede requerir librerías de compatibilidad en Alpine.
RUN apk add --no-cache ca-certificates libc6-compat

# CRÍTICO: Crear el directorio /src y copiar TODO el código fuente para que al depurar desde VSCode se reconozcan los simbolos
WORKDIR /src
COPY --from=build /src/ /src/

# Copiar delve y el binario compilado
COPY --from=build /go/bin/dlv /usr/local/bin/dlv
COPY --from=build /out/multiplicador-operator /usr/local/bin/multiplicador-operator
RUN chmod +x /usr/local/bin/multiplicador-operator /usr/local/bin/dlv

# Indicamos explicitamente que el puerto 40000 es el que usaremos para la depuración remota con Delve. Esto es importante para que Docker sepa que este puerto estará en uso y pueda mapearlo correctamente cuando ejecutemos el contenedor.
EXPOSE 40000

# El ENTRYPOINT inicia Delve en modo headless (--headless=true), escuchando en el puerto 40000 (--listen=:40000), y ejecuta el binario con exec. Esto permite que hagamos conexiones remotas desde VS Code.
# Ejecutar con API version 2 y accept-multiclient para mejor compatibilidad con VSCode. Con multiclient lo que hacemos es permitir que múltiples sesiones de debug se conecten al mismo contenedor, lo cual es útil si quieres tener varias instancias del operador corriendo o si quieres reconectar sin reiniciar el contenedor.
ENTRYPOINT ["/usr/local/bin/dlv", \
    "exec", \
    "/usr/local/bin/multiplicador-operator", \
    "--headless=true", \
    "--listen=:40000", \
    "--api-version=2", \
    "--accept-multiclient", \
    "--log"]
```

compilamos y cargamos esta imagen como hicimos antes:

```ps
docker build -t multiplicador-operator:debug -f .\Dockerfile.depura .

kind load docker-image multiplicador-operator:debug --name desktop
```

Desplegamos esa imagen. He creado un script, `.\ejemplos-debug.ps1` para automatizar el despliegue. Destacar que cuando desplegamos el operador no se ejecuta, se ejecuta Delve, . Tendremos que hacer un port-forwarding para llegar a Delve:

```ps
kubectl port-forward deployment/multiplicador-operator 40000:40000 -n default
```
a continuación nos podemos conectar con delve, y arrancar la ejecución - hasta el primer breakpoint:

```ps
dlv connect localhost:40000
```

lanzamos el comando delve _continue_:

```ps
(dlv) continue
```

Podemos hacer que Delve arranque por defecto el operador, incluyendo la opción en el _ENTRYPOINT_ del Dockerfile:
 
```yaml
ENTRYPOINT ["/usr/local/bin/dlv",
"exec",
"/usr/local/bin/multiplicador-operator",
"--headless=true",
"--listen=:40000",
"--api-version=2",
"--accept-multiclient",
"--continue", # Delve arranca el operador
"--log"]
```