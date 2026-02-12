package main

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"
	kfake "k8s.io/client-go/kubernetes/fake"
)

func TestGetIntField(t *testing.T) {
	u := &unstructured.Unstructured{Object: map[string]interface{}{
		"spec": map[string]interface{}{
			"int64":   int64(7),
			"float64": float64(8.0),
		},
	}}

	if v, ok := getIntField(u, "spec", "int64"); !ok || v != 7 {
		t.Fatalf("expected int64 7, got %v,%v", v, ok)
	}
	if v, ok := getIntField(u, "spec", "float64"); !ok || v != 8 {
		t.Fatalf("expected float64->8, got %v,%v", v, ok)
	}
	if _, ok := getIntField(u, "spec", "missing"); ok {
		t.Fatalf("expected missing field to return ok=false")
	}
}

func TestMakeDeployment(t *testing.T) {
	d := makeDeployment("my-dep", "myns", 3, 5)
	if d.Name != "my-dep" || d.Namespace != "myns" {
		t.Fatalf("unexpected name/namespace: %s/%s", d.Name, d.Namespace)
	}
	if d.Spec.Replicas == nil || *d.Spec.Replicas != 3 {
		t.Fatalf("unexpected replicas: %v", d.Spec.Replicas)
	}
	if len(d.Spec.Template.Spec.Containers) == 0 {
		t.Fatalf("no containers in pod template")
	}
	var found bool
	for _, e := range d.Spec.Template.Spec.Containers[0].Env {
		if e.Name == "MULTIPLIER" {
			if e.Value != "5" {
				t.Fatalf("unexpected MULTIPLIER value: %s", e.Value)
			}
			found = true
		}
	}
	if !found {
		t.Fatalf("MULTIPLIER env var not found")
	}
}

func TestReconcileOneCreatesDeploymentAndUpdatesStatus(t *testing.T) {
	// initial unstructured CR
	u := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "gz.com/v1",
		"kind":       "Multiplicador",
		"metadata": map[string]interface{}{
			"name":      "ejemplo",
			"namespace": "default",
		    "uid":       "uid-1",
		},
		"spec": map[string]interface{}{
			"replicas":   int64(2),
			"multiplier": int64(3),
		},
	}}
	// fake dynamic client seeded with the CR
	dyn := fake.NewSimpleDynamicClient(runtime.NewScheme(), u.DeepCopy())
	// fake typed client
	clientset := kfake.NewSimpleClientset()

	ctx := context.Background()
	if err := reconcileOne(ctx, dyn, clientset, u); err != nil {
		t.Fatalf("reconcileOne failed: %v", err)
	}

	// check Deployment created
	d, err := clientset.AppsV1().Deployments("default").Get(ctx, "ejemplo-multiplicador", v1.GetOptions{})
	if err != nil {
		t.Fatalf("get deployment failed: %v", err)
	}
	if d.Spec.Replicas == nil || *d.Spec.Replicas != 2 {
		t.Fatalf("unexpected deployment replicas: %v", d.Spec.Replicas)
	}
	// check env var
	if len(d.Spec.Template.Spec.Containers) == 0 {
		t.Fatalf("deployment has no containers")
	}
	var multVal string
	for _, e := range d.Spec.Template.Spec.Containers[0].Env {
		if e.Name == "MULTIPLIER" {
			multVal = e.Value
		}
	}
	if multVal != "3" {
		t.Fatalf("unexpected MULTIPLIER in deployment: %q", multVal)
	}

	// check CR annotations and status updated in dynamic client
	u2, err := dyn.Resource(gvr).Namespace("default").Get(ctx, "ejemplo", v1.GetOptions{})
	if err != nil {
		t.Fatalf("get updated CR failed: %v", err)
	}
	ann := u2.GetAnnotations()
	if ann["multiplicador.gz.com/valid"] != "true" {
		t.Fatalf("expected valid annotation=true, got: %v", ann)
	}
	if ann["multiplicador.gz.com/multiplier"] != "3" {
		t.Fatalf("expected multiplier annotation=3, got: %v", ann)
	}
	// check status.replica(s)
	st, ok, _ := unstructured.NestedMap(u2.Object, "status")
	if !ok {
		t.Fatalf("status not found on CR")
	}
	// replicas might be stored as int64; be flexible
	r, found := st["replicas"]
	if !found {
		t.Fatalf("status.replicas not found")
	}
	switch v := r.(type) {
	case int64:
		if v != 2 {
			t.Fatalf("unexpected status.replicas: %v", v)
		}
	case float64:
		if int64(v) != 2 {
			t.Fatalf("unexpected status.replicas (float): %v", v)
		}
	default:
		t.Fatalf("unexpected type for status.replicas: %T", v)
	}
}

// tiny helper to assert owner reference exists on deployment (not used yet but handy)
func hasOwnerRef(d *appsv1.Deployment, uid string) bool {
	for _, or := range d.ObjectMeta.OwnerReferences {
		if string(or.UID) == uid {
			return true
		}
	}
	return false
}
