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