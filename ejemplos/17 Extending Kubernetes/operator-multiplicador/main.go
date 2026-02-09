package main

import (
	"context"
	"flag"
	"fmt"
	"log"
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
	gvr = schema.GroupVersionResource{Group: "example.com", Version: "v1", Resource: "multiplicadors"}
)

func buildConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	return rest.InClusterConfig()
}

func main() {
	var kubeconfig string
	flag.StringVar(&kubeconfig, "kubeconfig", "", "absolute path to the kubeconfig file")
	flag.Parse()

	cfg, err := buildConfig(kubeconfig)
	if err != nil {
		log.Fatalf("unable to build kubeconfig: %v", err)
	}

	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		log.Fatalf("unable to create dynamic client: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Fatalf("unable to create clientset: %v", err)
	}

	log.Println("Starting simple Multiplicador operator (polling reconciler)")

	for {
		if err := reconcileAll(context.TODO(), dyn, clientset); err != nil {
			log.Printf("reconcile error: %v", err)
		}
		time.Sleep(5 * time.Second)
	}
}

func reconcileAll(ctx context.Context, dyn dynamic.Interface, clientset *kubernetes.Clientset) error {
	list, err := dyn.Resource(gvr).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("list multiplicadors: %w", err)
	}

	for _, item := range list.Items {
		if err := reconcileOne(ctx, dyn, clientset, &item); err != nil {
			log.Printf("reconcile %s/%s failed: %v", item.GetNamespace(), item.GetName(), err)
		}
	}
	return nil
}

func getIntField(u *unstructured.Unstructured, fields ...string) (int64, bool) {
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

func reconcileOne(ctx context.Context, dyn dynamic.Interface, clientset *kubernetes.Clientset, u *unstructured.Unstructured) error {
	ns := u.GetNamespace()
	if ns == "" {
		ns = "default"
	}
	name := u.GetName()

	replicas64, ok := getIntField(u, "spec", "replicas")
	var replicas int32 = 1
	if ok {
		replicas = int32(replicas64)
	}

	multiplier64, ok := getIntField(u, "spec", "multiplier")
	var multiplier int64 = 2
	if ok {
		multiplier = multiplier64
	}

	if multiplier <= 0 {
		ann := u.GetAnnotations()
		if ann == nil {
			ann = map[string]string{}
		}
		ann["multiplicador.example.com/valid"] = "false"
		ann["multiplicador.example.com/error"] = "multiplier must be positive integer"
		u.SetAnnotations(ann)
		_, err := dyn.Resource(gvr).Namespace(ns).Update(ctx, u, metav1.UpdateOptions{})
		return err
	}

	ann := u.GetAnnotations()
	if ann == nil {
		ann = map[string]string{}
	}
	ann["multiplicador.example.com/valid"] = "true"
	ann["multiplicador.example.com/multiplier"] = fmt.Sprintf("%d", multiplier)
	u.SetAnnotations(ann)
	if _, err := dyn.Resource(gvr).Namespace(ns).Update(ctx, u, metav1.UpdateOptions{}); err != nil {
		log.Printf("warn: updating annotations failed: %v", err)
	}

	deplName := name + "-multiplicador"
	existing, err := clientset.AppsV1().Deployments(ns).Get(ctx, deplName, metav1.GetOptions{})
	if err != nil {
		d := makeDeployment(deplName, ns, replicas, multiplier)
		_, err := clientset.AppsV1().Deployments(ns).Create(ctx, d, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("create deployment: %w", err)
		}
		log.Printf("created Deployment %s/%s (replicas=%d multiplier=%d)", ns, deplName, replicas, multiplier)
		return nil
	}

	needUpdate := false
	if existing.Spec.Replicas == nil || *existing.Spec.Replicas != replicas {
		existing.Spec.Replicas = &replicas
		needUpdate = true
	}
	if len(existing.Spec.Template.Spec.Containers) > 0 {
		envs := existing.Spec.Template.Spec.Containers[0].Env
		found := false
		for i := range envs {
			if envs[i].Name == "MULTIPLIER" {
				if envs[i].Value != fmt.Sprintf("%d", multiplier) {
					envs[i].Value = fmt.Sprintf("%d", multiplier)
					existing.Spec.Template.Spec.Containers[0].Env = envs
					needUpdate = true
				}
				found = true
				break
			}
		}
		if !found {
			existing.Spec.Template.Spec.Containers[0].Env = append(existing.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{Name: "MULTIPLIER", Value: fmt.Sprintf("%d", multiplier)})
			needUpdate = true
		}
	}

	if needUpdate {
		if _, err := clientset.AppsV1().Deployments(ns).Update(ctx, existing, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("update deployment: %w", err)
		}
		log.Printf("updated Deployment %s/%s (replicas=%d multiplier=%d)", ns, deplName, replicas, multiplier)
	}
	return nil
}

func makeDeployment(name, namespace string, replicas int32, multiplier int64) *appsv1.Deployment {
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
							Env:   []corev1.EnvVar{{Name: "MULTIPLIER", Value: fmt.Sprintf("%d", multiplier)}},
							ReadinessProbe: &corev1.Probe{
								Handler:             corev1.Handler{HTTPGet: &corev1.HTTPGetAction{Path: "/health", Port: intstr.FromInt(8080)}},
								InitialDelaySeconds: 3,
								PeriodSeconds:       5,
							},
						},
					},
				},
			},
		},
	}
}
