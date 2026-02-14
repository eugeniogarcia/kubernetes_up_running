package main

// Multiplicador operator (simple polling reconciler)
//
// This binary watches custom resources of GroupVersionResource defined by
// `gvr` (gz.com/v1, resource "multiplicadores"). For each CR it ensures
// there is a corresponding Deployment named <cr-name>-multiplicador whose
// replicas and environment variable MULTIPLIER reflect the CR's spec.
//
// The implementation used here is intentionally minimal and educational:
// - It uses a polling loop (reconcileAll invoked periodically) rather than
//   informers/watch-based controllers.
// - It uses the dynamic client to read/write unstructured CR objects and the
//   typed clientset to manage core resources (Deployments).
//
// The comments above each function explain its purpose and how it contributes
// to the reconcile loop and the behavior expected from a Kubernetes controller
// (owner references, status updates, idempotent reconciliation).

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"reflect"
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
	gvr = schema.GroupVersionResource{Group: "gz.com", Version: "v1", Resource: "multiplicadores"}
)

func buildConfig(kubeconfig string) (*rest.Config, error) {
	// buildConfig returns a *rest.Config suitable for creating client-go
	// clients. When running locally you pass `--kubeconfig` to point to your
	// kubeconfig file; when running in-cluster the function falls back to
	// InClusterConfig which uses the Pod's ServiceAccount token.
	if kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	return rest.InClusterConfig()
}

func ensureServiceAndProxy(ctx context.Context, dyn dynamic.Interface, clientset kubernetes.Interface, u *unstructured.Unstructured, deplName, ruta, host string) error {
	ns := u.GetNamespace()
	if ns == "" {
		ns = "default"
	}
	svcName := deplName + "-svc"
	labels := map[string]string{"app": "multiplicador", "multiplicador-name": deplName}

	// Ensure Service
	svc, err := clientset.CoreV1().Services(ns).Get(ctx, svcName, metav1.GetOptions{})
	if err != nil {
		s := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: svcName, Namespace: ns, Labels: labels},
			Spec: corev1.ServiceSpec{
				Selector: labels,
				Ports:    []corev1.ServicePort{{Port: 8080, TargetPort: intstr.FromInt(8080)}},
			},
		}
		// set ownerref so Service is garbage-collected with the CR
		gvk := u.GroupVersionKind()
		if gvk.Empty() {
			gvk = schema.GroupVersionKind{Group: gvr.Group, Version: gvr.Version, Kind: "Multiplicador"}
		}
		s.ObjectMeta.OwnerReferences = append(s.ObjectMeta.OwnerReferences, *metav1.NewControllerRef(u, gvk))
		if _, err := clientset.CoreV1().Services(ns).Create(ctx, s, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("create service: %w", err)
		}
	} else {
		// ensure selector/ports and ownerref
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
	// Use CR name as HTTPProxy name (user request)
	proxyName := u.GetName()
	svcRef := map[string]interface{}{"name": svcName, "port": int64(8080), "prefixRewrite": "/multiplica"}

	// Build proxy spec. If host provided, set virtualhost.fqdn
	proxySpec := map[string]interface{}{
		"routes": []interface{}{
			map[string]interface{}{
				"conditions": []interface{}{map[string]interface{}{"prefix": "/" + ruta}},
				"services":   []interface{}{svcRef},
			},
		},
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

	// Ensure OwnerReference on proxy so it is garbage-collected with the CR
	gvk := u.GroupVersionKind()
	if gvk.Empty() {
		gvk = schema.GroupVersionKind{Group: gvr.Group, Version: gvr.Version, Kind: "Multiplicador"}
	}
	ownerRef := metav1.NewControllerRef(u, gvk)
	ownerMap := map[string]interface{}{
		"apiVersion": ownerRef.APIVersion,
		"kind":       ownerRef.Kind,
		"name":       ownerRef.Name,
		"uid":        string(ownerRef.UID),
		"controller": true,
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
		return nil
	}

	// existing proxy: ensure ownerRef present and spec up-to-date
	updated := false
	// ensure ownerRefs
	meta, _ := existingProxy.Object["metadata"].(map[string]interface{})
	ors, _ := meta["ownerReferences"].([]interface{})
	found := false
	for _, or := range ors {
		if om, ok := or.(map[string]interface{}); ok {
			if om["uid"] == string(ownerRef.UID) {
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

	// ensure spec match
	if !reflect.DeepEqual(existingProxy.Object["spec"], proxySpec) {
		existingProxy.Object["spec"] = proxySpec
		updated = true
	}

	if updated {
		if _, err := dyn.Resource(proxyGVR).Namespace(ns).Update(ctx, existingProxy, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("update httpproxy: %w", err)
		}
	}

	return nil
}

func main() {
	var kubeconfig string
	flag.StringVar(&kubeconfig, "kubeconfig", "", "absolute path to the kubeconfig file")
	flag.Parse()
	cfg, err := buildConfig(kubeconfig)
	if err != nil {
		log.Fatalf("No se pudo construir la configuración kubeconfig: %v", err)
	}

	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		log.Fatalf("No se pudo crear el cliente dinámico: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Fatalf("No se pudo crear el clientset: %v", err)
	}

	log.Println("Arrancado el operador Multiplicador (polling reconciler)")

	// Crear un contexto cancelable que se cancelará en SIGINT/SIGTERM para que
	// el operador pueda apagarse de manera ordenada. Usar un ticker evita
	// dormir dentro de los bucles de reconciliación y hace que el apagado sea
	// sensible.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Bucle principal de control: llamar a reconcileAll periódicamente. Esta es la forma más simple
	// de un bucle de controlador: se basa en sondeos y, por lo tanto, siempre es
	// eventualmente consistente, pero no tan eficiente o sensible como un
	// controlador basado en informadores/vigilancia.
	for {
		if err := reconcileAll(ctx, dyn, clientset); err != nil {
			log.Printf("error de reconciliación: %v", err)
		}

		select {
		case <-ctx.Done():
			log.Println("apagando el operador multiplicador")
			return
		case <-ticker.C:
			// continue to next reconciliation
		}
	}
}

func reconcileAll(ctx context.Context, dyn dynamic.Interface, clientset kubernetes.Interface) error {
	// reconcileAll lista todos los recursos personalizados del tipo objetivo y llama
	// reconcileOne para cada uno. En un controlador de producción normalmente
	// usarías informadores y colas de trabajo para que cada cambio desencadene una reconciliación
	// para el objeto específico; aquí lo mantenemos simple y hacemos una lista completa
	// en cada pasada (controlador por sondeo).
	list, err := dyn.Resource(gvr).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("lista multiplicadores: %w", err)
	}

	for _, item := range list.Items {
		// reconcileOne es responsable de hacer que el estado externo (Deployment)
		// coincida con el estado deseado declarado en el CR. Los errores se registran por
		// objeto para evitar detener la reconciliación de otros objetos.
		if err := reconcileOne(ctx, dyn, clientset, &item); err != nil {
			log.Printf("reconciliar %s/%s falló: %v", item.GetNamespace(), item.GetName(), err)
		}
	}
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

func reconcileOne(ctx context.Context, dyn dynamic.Interface, clientset kubernetes.Interface, u *unstructured.Unstructured) error {
	ns := u.GetNamespace()
	if ns == "" {
		ns = "default"
	}
	name := u.GetName()

	// Leer replicas deseadas y multiplicador desde `spec` del CR. Se proporcionan valores predeterminados si los campos están ausentes para que el reconciliador sea tolerante con CR mínimos. En un operador de producción, es posible que desees hacer cumplir la validación del esquema a través de un CRD y/o usar tipos más estructurados en lugar de no estructurados, pero aquí lo mantenemos simple y flexible.
	replicas64, ok := getIntField(u, "spec", "replicas")
	var replicas int32 = 1
	if ok {
		replicas = int32(replicas64)
	}
	multiplicador64, ok := getIntField(u, "spec", "multiplicador")
	var multiplicador int64 = 2
	if ok {
		multiplicador = multiplicador64
	}
	ruta, _ := getStringField(u, "spec", "ruta")
	host, _ := getStringField(u, "spec", "host")

	if multiplicador <= 0 {
		// If the CR declares an invalid multiplier we annotate and set a
		// status to indicate the problem. Note: a production operator would
		// probably use the CR `status` subresource and more structured
		// conditions; here we keep it simple.
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

	// Mark the CR as valid and record the multiplier used in annotations.
	// Annotations are easy to inspect but `status` is preferable for machine
	// readable state; we update both below.
	ann := u.GetAnnotations()
	if ann == nil {
		ann = map[string]string{}
	}
	ann["multiplicador.gz.com/valid"] = "true"
	ann["multiplicador.gz.com/multiplicador"] = fmt.Sprintf("%d", multiplicador)
	u.SetAnnotations(ann)
	if _, err := dyn.Resource(gvr).Namespace(ns).Update(ctx, u, metav1.UpdateOptions{}); err != nil {
		log.Printf("warn: updating annotations failed: %v", err)
	}

	deplName := name + "-multiplicador"
	existing, err := clientset.AppsV1().Deployments(ns).Get(ctx, deplName, metav1.GetOptions{})
	if err != nil {
		d := makeDeployment(deplName, ns, replicas, multiplicador)
		// ensure OwnerReference so Deployment is garbage-collected with the CR
		gvk := u.GroupVersionKind()
		if gvk.Empty() {
			gvk = schema.GroupVersionKind{Group: gvr.Group, Version: gvr.Version, Kind: "Multiplicador"}
		}
		ownerRef := metav1.NewControllerRef(u, gvk)
		d.ObjectMeta.OwnerReferences = append(d.ObjectMeta.OwnerReferences, *ownerRef)

		_, err := clientset.AppsV1().Deployments(ns).Create(ctx, d, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("create deployment: %w", err)
		}
		log.Printf("created Deployment %s/%s (replicas=%d multiplicador=%d)", ns, deplName, replicas, multiplicador)
		// update status on the CR to reflect created state
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
		// ensure Service and HTTPProxy
		if err := ensureServiceAndProxy(ctx, dyn, clientset, u, deplName, ruta, host); err != nil {
			return fmt.Errorf("ensure service/proxy: %w", err)
		}
		return nil
	}

	needUpdate := false
	if existing.Spec.Replicas == nil || *existing.Spec.Replicas != replicas {
		existing.Spec.Replicas = &replicas
		needUpdate = true
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
	// makeDeployment constructs the Deployment that the operator manages for
	// each CR. The Deployment is labelled so it can be selected, and it uses
	// the MULTIPLIER environment variable to pass the CR's multiplier value
	// into the container.
	labels := map[string]string{"app": "multiplicador", "multiplicador-name": name}
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
