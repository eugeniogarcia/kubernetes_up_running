package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"reflect"
	"strings"
	"syscall"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	// referencia a nuestro recurso
	gvr           = schema.GroupVersionResource{Group: "gz.com", Version: "v1", Resource: "multiplicadores"}
	finalizerName = "multiplicador.gz.com/finalizer"
)

// configuracion del cliente rest go
func buildConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	return rest.InClusterConfig()
}

// Se necesitan ambos porque el operador debe gestionar tanto recursos core estándar (Deployments, Services)
// como recursos personalizados definidos por el CRD del operador (Multiplicador).
//
// El operador utiliza un enfoque simple de polling-based reconciliation: ejecuta reconcileAll() cada 5 segundos
// para verificar el estado deseado vs. estado actual. Está diseñado para apagarse ordenadamente ante
// SIGINT/SIGTERM usando un contexto cancelable.
func main() {
	var kubeconfig string
	flag.StringVar(&kubeconfig, "kubeconfig", "", "absolute path to the kubeconfig file")
	flag.Parse()

	// obtiene la configuración para crear el cliente...
	cfg, err := buildConfig(kubeconfig)
	if err != nil {
		log.Fatalf("No se pudo construir la configuración kubeconfig: %v", err)
	}

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

	log.Println("Arrancado el operador Multiplicador (polling reconciler)")

	// creamos un contexto que permita la cancelación con SIGINT/SIGTERM para que el operador pueda apagarse de manera ordenada.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	// aseguramos que el contexto se cancele y los recursos se limpien al salir
	defer stop()
	// un ticker para ejecutar reconcileAll cada 5 segundos
	ticker := time.NewTicker(5 * time.Second)
	// aseguramos que el ticker se detenga al salir para liberar recursos
	defer ticker.Stop()

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
}

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

// tenemos como argumento el contexto, la api dinámica para interactuar con el api-server con cursos custom, el cliente para interactuar con recursos estandar, y el recurso que estamos verificano/conciliando
func reconcileOne(ctx context.Context, dyn dynamic.Interface, clientset kubernetes.Interface, u *unstructured.Unstructured) error {
	// obtiene el namespace del recurso que estamos reconnciliando
	ns := u.GetNamespace()
	if ns == "" {
		ns = "default"
	}
	// obtiene el nombre del recurso que estamos reconnciliando
	name := u.GetName()

	// Leer replicas deseadas y multiplicador desde `spec` del CR. Se proporcionan valores predeterminados si los campos están ausentes para que el reconciliador sea tolerante con CR mínimos. En un operador de producción, es posible que desees hacer cumplir la validación del esquema a través de un CRD y/o usar tipos más estructurados en lugar de no estructurados, pero aquí lo mantenemos simple y flexible.

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

	// If the CR is being deleted, perform cleanup (remove owner reference and route from proxy)
	if u.GetDeletionTimestamp() != nil {
		if err := cleanupProxyForCR(ctx, dyn, u); err != nil {
			log.Printf("warning: cleanup failed for %s/%s: %v", u.GetNamespace(), u.GetName(), err)
		}
		// remove finalizer so deletion can proceed
		finalizers := u.GetFinalizers()
		nf := []string{}
		for _, f := range finalizers {
			if f == finalizerName {
				continue
			}
			nf = append(nf, f)
		}
		if len(nf) != len(finalizers) {
			u.SetFinalizers(nf)
			_, _ = dyn.Resource(gvr).Namespace(ns).Update(ctx, u, metav1.UpdateOptions{})
		}
		return nil
	}

	if multiplicador <= 0 {
		// indicamos en una anotación el error
		ann := u.GetAnnotations()
		if ann == nil {
			ann = map[string]string{}
		}
		ann["multiplicador.gz.com/valid"] = "false"
		ann["multiplicador.gz.com/error"] = "el multiplicador tiene que ser positivo"
		u.SetAnnotations(ann)

		// also set status to reflect invalid state
		if err := unstructured.SetNestedField(u.Object, map[string]interface{}{
			"valid": false,
			"error": "el multiplicador tiene que ser positivo",
		}, "status"); err != nil {
			log.Printf("warn: set status failed: %v", err)
		}
		if _, err := dyn.Resource(gvr).Namespace(ns).UpdateStatus(ctx, u, metav1.UpdateOptions{}); err != nil {
			// fallback to Update if UpdateStatus not supported by the CRD
			if _, err2 := dyn.Resource(gvr).Namespace(ns).Update(ctx, u, metav1.UpdateOptions{}); err2 != nil {
				return err2
			}
		}
		return nil
	}

	// obtenemos las anotaciones del recurso que estamos reconciliado
	ann := u.GetAnnotations()
	if ann == nil {
		ann = map[string]string{}
	}

	// añadimos un par de anotaciones al recurso que estamos reconciliando
	ann["multiplicador.gz.com/valid"] = "true"
	ann["multiplicador.gz.com/multiplicador"] = fmt.Sprintf("%d", multiplicador)
	u.SetAnnotations(ann)
	if _, err := dyn.Resource(gvr).Namespace(ns).Update(ctx, u, metav1.UpdateOptions{}); err != nil {
		log.Printf("warn: updating annotations failed: %v", err)
	}

	// determina el nombre del deployment
	deplName := name + "-multiplicador"
	// usamos el cliente para interrogar al api server y comprobar el el deployment ya existe o no. Si no existe, lo creamos. Si existe, comprobamos si su estado coincide con el deseado (replicas y variable de entorno) y lo actualizamos si es necesario.
	existing, err := clientset.AppsV1().Deployments(ns).Get(ctx, deplName, metav1.GetOptions{})
	// si el deployment no existe, lo creamos
	if err != nil {
		// creamos un deployment
		d := makeDeployment(deplName, ns, replicas, multiplicador)
		// vamos a establecer una referencia del deployment con el custom resource (CR), de modo que cuando el se elimine se eliminen en cascada todos los recursos asociados (deployment, service, httpproxy). Para eso usamos OwnerReferences, que es un mecanismo de Kubernetes para establecer relaciones de propiedad entre objetos. Al establecer el OwnerReference del deployment apuntando al CR, le indicamos a Kubernetes que el deployment es "propiedad" del CR, y que si el CR se elimina, Kubernetes también eliminará automáticamente el deployment. Esto ayuda a mantener el cluster limpio y evita recursos huérfanos.
		// obtenermos la referencia al recurso que estamos reconciliando
		gvk := u.GroupVersionKind()
		if gvk.Empty() {
			gvk = schema.GroupVersionKind{Group: gvr.Group, Version: gvr.Version, Kind: "Multiplicador"}
		}
		ownerRef := metav1.NewControllerRef(u, gvk)
		// fija la referencia en el deployment. Esto relaciona a ojos de kubernetes el deployment con el recurso que estamos reconciliando, de modo que si el recurso se elimina, el deployment se eliminará automáticamente. Podemos tener más de un owner, asun que lo que hacemos es un append para añadir el nuevo owner
		d.ObjectMeta.OwnerReferences = append(d.ObjectMeta.OwnerReferences, *ownerRef)

		// creamos el deployment en el cluster
		_, err := clientset.AppsV1().Deployments(ns).Create(ctx, d, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("create deployment: %w", err)
		}
		log.Printf("created Deployment %s/%s (replicas=%d multiplicador=%d)", ns, deplName, replicas, multiplicador)

		// actualizamos el estado del recurso que estamos reconciliando para reflejar que ya hemos creado el deployment y que el recurso está listo. El estado es un subrecurso especial que se utiliza para reflejar el estado actual del recurso, y que suele ser actualizado por el controlador para informar sobre el estado de la aplicación o recurso que está gestionando. En este caso, actualizamos el estado para indicar que el recurso está "ready" (listo) y para reflejar el número de replicas y el multiplicador actual.
		if err := unstructured.SetNestedField(u.Object, map[string]interface{}{
			"ready":         true,
			"replicas":      int64(replicas),
			"multiplicador": multiplicador,
		}, "status"); err != nil {
			log.Printf("aviso: set status fallo: %v", err)
		} else {
			if _, err := dyn.Resource(gvr).Namespace(ns).UpdateStatus(ctx, u, metav1.UpdateOptions{}); err != nil {
				if _, err2 := dyn.Resource(gvr).Namespace(ns).Update(ctx, u, metav1.UpdateOptions{}); err2 != nil {
					log.Printf("aviso: update status fallback fallo: %v", err2)
				}
			}
		}
		// nos encargamos de crear el servicio y el HTTPProxy
		if err := ensureServiceAndProxy(ctx, dyn, clientset, u, deplName, ruta, host); err != nil {
			return fmt.Errorf("ensure service/proxy: %w", err)
		}
		return nil
	}

	// en este punto ya tenemos los recursos estandar (deployment, service, httpproxy) creados para el custom resource que estamos reconciliando, así que ahora lo que hacemos es verificar si el estado actual de esos recursos coincide con el estado deseado (replicas y multiplicador) y actualizarlos si es necesario. Esto es importante porque el usuario puede modificar el custom resource después de creado, y el operador debe asegurarse de que los recursos asociados se mantengan en el estado correcto según la especificación del custom resource.
	needUpdate := false
	if existing.Spec.Replicas == nil || *existing.Spec.Replicas != replicas {
		existing.Spec.Replicas = &replicas
		needUpdate = true // hay que actualizar el número de replicas
	}
	// ensure OwnerReference exists on existing Deployment
	gvk := u.GroupVersionKind()
	if gvk.Empty() {
		gvk = schema.GroupVersionKind{Group: gvr.Group, Version: gvr.Version, Kind: "Multiplicador"}
	}
	desiredOwner := *metav1.NewControllerRef(u, gvk)
	foundOwner := false
	for _, or := range existing.ObjectMeta.OwnerReferences {
		if or.UID == desiredOwner.UID {
			foundOwner = true
			break
		}
	}
	if !foundOwner {
		existing.ObjectMeta.OwnerReferences = append(existing.ObjectMeta.OwnerReferences, desiredOwner)
		needUpdate = true
	}
	if len(existing.Spec.Template.Spec.Containers) > 0 {
		envs := existing.Spec.Template.Spec.Containers[0].Env
		found := false
		for i := range envs {
			if envs[i].Name == "MULTIPLIER" {
				if envs[i].Value != fmt.Sprintf("%d", multiplicador) {
					envs[i].Value = fmt.Sprintf("%d", multiplicador)
					existing.Spec.Template.Spec.Containers[0].Env = envs
					needUpdate = true
				}
				found = true
				break
			}
		}
		if !found {
			existing.Spec.Template.Spec.Containers[0].Env = append(existing.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{Name: "MULTIPLIER", Value: fmt.Sprintf("%d", multiplicador)})
			needUpdate = true
		}
	}

	// actualizamos el deployment si es necesario
	if needUpdate {
		if _, err := clientset.AppsV1().Deployments(ns).Update(ctx, existing, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("update deployment: %w", err)
		}
		log.Printf("updated Deployment %s/%s (replicas=%d multiplicador=%d)", ns, deplName, replicas, multiplicador)
	}
	// ensure Service and HTTPProxy reflect current desired state
	if err := ensureServiceAndProxy(ctx, dyn, clientset, u, deplName, ruta, host); err != nil {
		return fmt.Errorf("ensure service/proxy: %w", err)
	}
	// after successful reconciliation, update status
	if err := unstructured.SetNestedField(u.Object, map[string]interface{}{
		"ready":         true,
		"replicas":      int64(replicas),
		"multiplicador": multiplicador,
	}, "status"); err != nil {
		log.Printf("warn: set status failed: %v", err)
	} else {
		if _, err := dyn.Resource(gvr).Namespace(ns).UpdateStatus(ctx, u, metav1.UpdateOptions{}); err != nil {
			if _, err2 := dyn.Resource(gvr).Namespace(ns).Update(ctx, u, metav1.UpdateOptions{}); err2 != nil {
				log.Printf("warn: update status fallback failed: %v", err2)
			}
		}
	}
	return nil
}

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

func ensureServiceAndProxy(ctx context.Context, dyn dynamic.Interface, clientset kubernetes.Interface, u *unstructured.Unstructured, deplName, ruta, host string) error {
	// identifica el namespace del recurso que estamos reconciliando para crear el servicio en el mismo namespace. Si el recurso no tiene namespace, se asume "default"
	ns := u.GetNamespace()
	if ns == "" {
		ns = "default"
	}
	// especifica el nombre del servicio basado en el nombre del deployment
	svcName := deplName + "-svc"
	// etiquetas que vamos a utilizar. Un par de etiquetas
	labels := map[string]string{"app": "multiplicador", "multiplicador-name": deplName}

	// Comprobamos si el servicio ya existe. Si no existe, lo creamos. Si existe, verificamos que su configuración (selector, puertos) y OwnerReference sean correctos y lo actualizamos si es necesario.
	svc, err := clientset.CoreV1().Services(ns).Get(ctx, svcName, metav1.GetOptions{})
	if err != nil {
		s := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: svcName, Namespace: ns, Labels: labels},
			Spec: corev1.ServiceSpec{
				Selector: labels,
				Ports:    []corev1.ServicePort{{Port: 8080, TargetPort: intstr.FromInt(8080)}},
			},
		}
		// Especificamos que el servicio esta referenciado a un recurso a medida, el que estamos reconciliando, de modo que si el recurso se elimina, el servicio se eliminará automáticamente. Para eso usamos OwnerReferences, que es un mecanismo de Kubernetes para establecer relaciones de propiedad entre objetos. Al establecer el OwnerReference del servicio apuntando al CR, le indicamos a Kubernetes que el servicio es "propiedad" del CR, y que si el CR se elimina, Kubernetes también eliminará automáticamente el servicio. Esto ayuda a mantener el cluster limpio y evita recursos huérfanos.
		gvk := u.GroupVersionKind()
		if gvk.Empty() {
			gvk = schema.GroupVersionKind{Group: gvr.Group, Version: gvr.Version, Kind: "Multiplicador"}
		}
		s.ObjectMeta.OwnerReferences = append(s.ObjectMeta.OwnerReferences, *metav1.NewControllerRef(u, gvk))
		if _, err := clientset.CoreV1().Services(ns).Create(ctx, s, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("create service: %w", err)
		}
	} else {
		updated := false
		if !reflect.DeepEqual(svc.Spec.Selector, labels) {
			svc.Spec.Selector = labels
			updated = true
		}
		if len(svc.Spec.Ports) == 0 || svc.Spec.Ports[0].Port != 8080 {
			svc.Spec.Ports = []corev1.ServicePort{{Port: 8080, TargetPort: intstr.FromInt(8080)}}
			updated = true
		}
		gvk := u.GroupVersionKind()
		if gvk.Empty() {
			gvk = schema.GroupVersionKind{Group: gvr.Group, Version: gvr.Version, Kind: "Multiplicador"}
		}
		desiredOwner := *metav1.NewControllerRef(u, gvk)
		found := false
		for _, or := range svc.ObjectMeta.OwnerReferences {
			if or.UID == desiredOwner.UID {
				found = true
				break
			}
		}
		if !found {
			svc.ObjectMeta.OwnerReferences = append(svc.ObjectMeta.OwnerReferences, desiredOwner)
			updated = true
		}
		if updated {
			if _, err := clientset.CoreV1().Services(ns).Update(ctx, svc, metav1.UpdateOptions{}); err != nil {
				return fmt.Errorf("update service: %w", err)
			}
		}
	}

	// Ensure HTTPProxy (Contour) if ruta provided
	if ruta == "" {
		return nil
	}
	proxyGVR := schema.GroupVersionResource{Group: "projectcontour.io", Version: "v1", Resource: "httpproxies"}
	// Vamos a asegurar que no se creen dos http proxies para el mismo host, haciendo que el nombre del proxy sea univoco a partir del host.
	// El nombre del proxy por defecto será igual al nombre del recurso a reconciliar, pero vamos a usar el host para determina cual debe ser el nombre...
	// Además, soportamos limpieza: si el CR cambió de host/ruta, eliminaremos la ownerReference y la ruta antigua del proxy previo.
	ann := u.GetAnnotations()
	if ann == nil {
		ann = map[string]string{}
	}
	// Usamos una anotación para guardar en el propio recurso cual es el valor del host y de la ruta la ultima vez que el objeto a medida paso por las manos del operador. Si es la primera vez estarán en blanco
	prevProxy := ann["multiplicador.gz.com/proxy"]
	prevRoute := ann["multiplicador.gz.com/proxy-route"]

	proxyName := u.GetName()
	if host != "" {
		// el nombre del proxy se establece de forma univoca a partir del host, de modo que si varios recursos tienen el mismo host, compartirán el mismo proxy. Esto es importante porque Contour no permite tener dos HTTPProxies con el mismo virtualhost (host), asín que si queremos soportar esa funcionalidad, debemos asegurarnos de reutilizar el mismo HTTPProxy para los recursos que comparten host.
		proxyName = "vhost-" + strings.ReplaceAll(host, ".", "-")
	}

	// Si el proxy name previo es diferente del actual, significa que el host ha cambiado, así que tenemos que limpiar la referencia del proxy anterior (si existía) y eliminar la ruta antigua del proxy anterior (si existía). Esto es importante para evitar dejar rutas huérfanas en proxies antiguos que ya no corresponden al recurso que estamos reconciliando, y para evitar referencias incorrectas a proxies que ya no son relevantes para el recurso.
	if prevProxy != "" && prevProxy != proxyName {
		oldProxy, err := dyn.Resource(proxyGVR).Namespace(ns).Get(ctx, prevProxy, metav1.GetOptions{})
		if err == nil {
			// remove ownerRef matching this CR UID
			if meta, ok := oldProxy.Object["metadata"].(map[string]interface{}); ok {
				if ors, ok := meta["ownerReferences"].([]interface{}); ok {
					newOrs := []interface{}{}
					for _, or := range ors {
						if om, ok := or.(map[string]interface{}); ok {
							if om["uid"] == string(u.GetUID()) {
								continue
							}
						}
						newOrs = append(newOrs, or)
					}
					meta["ownerReferences"] = newOrs
					oldProxy.Object["metadata"] = meta
				}
			}
			// remove route matching prevRoute
			if spec, ok := oldProxy.Object["spec"].(map[string]interface{}); ok {
				if rts, ok := spec["routes"].([]interface{}); ok && prevRoute != "" {
					newRts := []interface{}{}
					for _, rr := range rts {
						keep := true
						if rm, ok := rr.(map[string]interface{}); ok {
							if conds, ok := rm["conditions"].([]interface{}); ok && len(conds) > 0 {
								if cm, ok := conds[0].(map[string]interface{}); ok {
									if p, ok := cm["prefix"].(string); ok && p == prevRoute {
										keep = false
									}
								}
							}
						}
						if keep {
							newRts = append(newRts, rr)
						}
					}
					spec["routes"] = newRts
					oldProxy.Object["spec"] = spec
				}
			}

			// decide whether to delete or update the old proxy
			deleteOld := false
			if meta, ok := oldProxy.Object["metadata"].(map[string]interface{}); ok {
				ors, _ := meta["ownerReferences"].([]interface{})
				if len(ors) == 0 {
					// if there are no owners left, and there are no routes, we can delete the proxy
					if spec, ok := oldProxy.Object["spec"].(map[string]interface{}); ok {
						if rts, ok := spec["routes"].([]interface{}); !ok || len(rts) == 0 {
							deleteOld = true
						}
					} else {
						deleteOld = true
					}
				}
			}
			if deleteOld {
				_ = dyn.Resource(proxyGVR).Namespace(ns).Delete(ctx, prevProxy, metav1.DeleteOptions{})
			} else {
				_, _ = dyn.Resource(proxyGVR).Namespace(ns).Update(ctx, oldProxy, metav1.UpdateOptions{})
			}
		}
	}

	// Especifica del httpproxy
	route := map[string]interface{}{
		"conditions":        []interface{}{map[string]interface{}{"prefix": "/" + ruta}},
		"services":          []interface{}{map[string]interface{}{"name": svcName, "port": int64(8080)}},
		"pathRewritePolicy": map[string]interface{}{"replacePrefix": []interface{}{map[string]interface{}{"replacement": "/multiplica"}}},
	}
	proxySpec := map[string]interface{}{
		"routes": []interface{}{route},
	}
	if host != "" {
		proxySpec["virtualhost"] = map[string]interface{}{"fqdn": host}
	}

	proxy := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "projectcontour.io/v1",
		"kind":       "HTTPProxy",
		"metadata":   map[string]interface{}{"name": proxyName, "namespace": ns},
		"spec":       proxySpec,
	}}

	// Vamos a especificar las referencias del proxy para que apunte al custom resource de modo que cuando se elimine el custom resource se elimine también el proxy. Sin embargo, como varios recursos pueden compartir el mismo proxy (si tienen el mismo host), no podemos usar una referencia de controlador (controller=true) porque solo puede haber un controlador por recurso. En su lugar, vamos a usar una referencia de propietario no controlador (controller=false) para establecer la relación entre el proxy y el recurso que estamos reconciliando. De esta manera, si el recurso se elimina, Kubernetes eliminará el proxy automáticamente, pero no habrá conflicto si varios recursos comparten el mismo proxy.
	gvk := u.GroupVersionKind()
	if gvk.Empty() {
		gvk = schema.GroupVersionKind{Group: gvr.Group, Version: gvr.Version, Kind: "Multiplicador"}
	}
	ownerMap := map[string]interface{}{
		"apiVersion": gvk.GroupVersion().String(),
		"kind":       gvk.Kind,
		"name":       u.GetName(),
		"uid":        string(u.GetUID()),
		"controller": false, // no es un controlador, solo una referencia de propietario para eliminación en cascada
	}

	// Try to get existing proxy
	existingProxy, err := dyn.Resource(proxyGVR).Namespace(ns).Get(ctx, proxyName, metav1.GetOptions{})
	if err != nil {
		// attach ownerRef to proxy metadata before create
		meta := proxy.Object["metadata"].(map[string]interface{})
		meta["ownerReferences"] = []interface{}{ownerMap}
		proxy.Object["metadata"] = meta
		if _, err := dyn.Resource(proxyGVR).Namespace(ns).Create(ctx, proxy, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("create httpproxy: %w", err)
		}
		// guardamos en anotaciones el host (el proxyName es univoco con el host) y la ruta
		ann["multiplicador.gz.com/proxy"] = proxyName
		ann["multiplicador.gz.com/proxy-route"] = "/" + ruta
		u.SetAnnotations(ann)
		_, _ = dyn.Resource(gvr).Namespace(ns).Update(ctx, u, metav1.UpdateOptions{})
		return nil
	}

	// existing proxy: ensure ownerRef present and merge route instead of replacing entire spec
	updated := false
	meta, _ := existingProxy.Object["metadata"].(map[string]interface{})
	ors, _ := meta["ownerReferences"].([]interface{})
	found := false
	for _, or := range ors {
		if om, ok := or.(map[string]interface{}); ok {
			if om["uid"] == string(u.GetUID()) {
				found = true
				break
			}
		}
	}
	if !found {
		meta["ownerReferences"] = append(ors, ownerMap)
		existingProxy.Object["metadata"] = meta
		updated = true
	}

	// merge routes: first, if the previous route referenced the same proxy but different prefix, remove the old prefix
	spec, _ := existingProxy.Object["spec"].(map[string]interface{})
	if prevProxy == proxyName && prevRoute != "" && prevRoute != ("/"+ruta) {
		if spec != nil {
			if rts, ok := spec["routes"].([]interface{}); ok {
				newRts := []interface{}{}
				for _, rr := range rts {
					remove := false
					if rm, ok := rr.(map[string]interface{}); ok {
						if conds, ok := rm["conditions"].([]interface{}); ok && len(conds) > 0 {
							if cm, ok := conds[0].(map[string]interface{}); ok {
								if p, ok := cm["prefix"].(string); ok && p == prevRoute {
									remove = true
								}
							}
						}
					}
					if !remove {
						newRts = append(newRts, rr)
					}
				}
				spec["routes"] = newRts
				existingProxy.Object["spec"] = spec
				updated = true
			}
		}
	}

	// merge routes: append our route if a route with same prefix is not present
	spec = existingProxy.Object["spec"].(map[string]interface{})
	var routes []interface{}
	if spec != nil {
		if r, ok := spec["routes"].([]interface{}); ok {
			routes = r
		}
	}
	// check if our prefix already exists
	prefix := "/" + ruta
	exists := false
	for i, r := range routes {
		if rm, ok := r.(map[string]interface{}); ok {
			if conds, ok := rm["conditions"].([]interface{}); ok && len(conds) > 0 {
				if cm, ok := conds[0].(map[string]interface{}); ok {
					if p, ok := cm["prefix"].(string); ok && p == prefix {
						// replace this route with desired one
						routes[i] = route
						exists = true
						break
					}
				}
			}
		}
	}
	if !exists {
		routes = append(routes, route)
	}
	if spec == nil {
		spec = map[string]interface{}{}
	}
	spec["routes"] = routes
	if host != "" {
		spec["virtualhost"] = map[string]interface{}{"fqdn": host}
	}
	existingProxy.Object["spec"] = spec
	updated = true

	if updated {
		if _, err := dyn.Resource(proxyGVR).Namespace(ns).Update(ctx, existingProxy, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("update httpproxy: %w", err)
		}
	}

	// persist annotations pointing to the proxy and route we ensured
	ann["multiplicador.gz.com/proxy"] = proxyName
	ann["multiplicador.gz.com/proxy-route"] = "/" + ruta
	u.SetAnnotations(ann)
	// ensure finalizer present so we can cleanup on delete
	finals := u.GetFinalizers()
	hasFinal := false
	for _, f := range finals {
		if f == finalizerName {
			hasFinal = true
			break
		}
	}
	if !hasFinal {
		finals = append(finals, finalizerName)
		u.SetFinalizers(finals)
	}
	_, _ = dyn.Resource(gvr).Namespace(ns).Update(ctx, u, metav1.UpdateOptions{})

	return nil
}

func getIntField(u *unstructured.Unstructured, fields ...string) (int64, bool) {
	// Helper to read an integer-like nested field from an unstructured
	// object (CR). The dynamic/unstructured representation often yields
	// numbers as float64 so we handle common numeric types and coerce them
	// to int64. Returns (0,false) when the field is missing or not numeric.
	v, found, _ := unstructured.NestedFieldNoCopy(u.Object, fields...)
	if !found || v == nil {
		return 0, false
	}
	switch t := v.(type) {
	case int64:
		return t, true
	case int:
		return int64(t), true
	case float64:
		return int64(t), true
	default:
		return 0, false
	}
}

func getStringField(u *unstructured.Unstructured, fields ...string) (string, bool) {
	v, found, _ := unstructured.NestedString(u.Object, fields...)
	if !found {
		return "", false
	}
	return v, true
}

// cleanupProxyForCR removes ownerReference and route entries for this CR's recorded proxy.
func cleanupProxyForCR(ctx context.Context, dyn dynamic.Interface, u *unstructured.Unstructured) error {
	ns := u.GetNamespace()
	if ns == "" {
		ns = "default"
	}
	ann := u.GetAnnotations()
	if ann == nil {
		return nil
	}
	prevProxy := ann["multiplicador.gz.com/proxy"]
	prevRoute := ann["multiplicador.gz.com/proxy-route"]
	if prevProxy == "" {
		return nil
	}
	proxyGVR := schema.GroupVersionResource{Group: "projectcontour.io", Version: "v1", Resource: "httpproxies"}
	oldProxy, err := dyn.Resource(proxyGVR).Namespace(ns).Get(ctx, prevProxy, metav1.GetOptions{})
	if err != nil {
		return nil
	}
	// remove ownerRef matching this CR UID
	if meta, ok := oldProxy.Object["metadata"].(map[string]interface{}); ok {
		if ors, ok := meta["ownerReferences"].([]interface{}); ok {
			newOrs := []interface{}{}
			for _, or := range ors {
				if om, ok := or.(map[string]interface{}); ok {
					if om["uid"] == string(u.GetUID()) {
						continue
					}
				}
				newOrs = append(newOrs, or)
			}
			meta["ownerReferences"] = newOrs
			oldProxy.Object["metadata"] = meta
		}
	}
	// remove route matching prevRoute
	if spec, ok := oldProxy.Object["spec"].(map[string]interface{}); ok {
		if rts, ok := spec["routes"].([]interface{}); ok && prevRoute != "" {
			newRts := []interface{}{}
			for _, rr := range rts {
				keep := true
				if rm, ok := rr.(map[string]interface{}); ok {
					if conds, ok := rm["conditions"].([]interface{}); ok && len(conds) > 0 {
						if cm, ok := conds[0].(map[string]interface{}); ok {
							if p, ok := cm["prefix"].(string); ok && p == prevRoute {
								keep = false
							}
						}
					}
				}
				if keep {
					newRts = append(newRts, rr)
				}
			}
			spec["routes"] = newRts
			oldProxy.Object["spec"] = spec
		}
	}

	// decide whether to delete or update the old proxy
	deleteOld := false
	if meta, ok := oldProxy.Object["metadata"].(map[string]interface{}); ok {
		ors, _ := meta["ownerReferences"].([]interface{})
		if len(ors) == 0 {
			if spec, ok := oldProxy.Object["spec"].(map[string]interface{}); ok {
				if rts, ok := spec["routes"].([]interface{}); !ok || len(rts) == 0 {
					deleteOld = true
				}
			} else {
				deleteOld = true
			}
		}
	}
	if deleteOld {
		_ = dyn.Resource(proxyGVR).Namespace(ns).Delete(ctx, prevProxy, metav1.DeleteOptions{})
	} else {
		_, _ = dyn.Resource(proxyGVR).Namespace(ns).Update(ctx, oldProxy, metav1.UpdateOptions{})
	}
	return nil
}
