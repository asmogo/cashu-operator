package controller

import (
	"context"
	"errors"
	"strings"
	"testing"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	mintv1alpha1 "github.com/asmogo/cashu-operator/api/v1alpha1"
	"github.com/asmogo/cashu-operator/internal/controller/generators"
)

func TestHandleError(t *testing.T) {
	ctx := context.Background()
	mint := unitTestCashuMint("error-handling")

	t.Run("conflict errors requeue immediately", func(t *testing.T) {
		reconciler, _ := newUnitTestReconciler(t, mint.DeepCopy())
		result, err := reconciler.handleError(ctx, mint.DeepCopy(), apierrors.NewConflict(resourceForTests(), mint.Name, errors.New("conflict")))
		if err != nil {
			t.Fatalf("handleError() error = %v, want nil", err)
		}
		if result.RequeueAfter == 0 {
			t.Fatal("handleError() should request an immediate requeue on conflicts")
		}
	})

	t.Run("not found errors requeue after a short delay", func(t *testing.T) {
		reconciler, _ := newUnitTestReconciler(t, mint.DeepCopy())
		result, err := reconciler.handleError(ctx, mint.DeepCopy(), apierrors.NewNotFound(resourceForTests(), "dependency"))
		if err != nil {
			t.Fatalf("handleError() error = %v, want nil", err)
		}
		if result.RequeueAfter != NotReadyRetryInterval {
			t.Fatalf("RequeueAfter = %s, want %s", result.RequeueAfter, NotReadyRetryInterval)
		}
	})

	t.Run("generic errors mark the resource failed", func(t *testing.T) {
		mintCopy := mint.DeepCopy()
		reconciler, client := newUnitTestReconciler(t, mintCopy)
		result, err := reconciler.handleError(ctx, mintCopy, errors.New("boom"))
		if err == nil {
			t.Fatal("handleError() error = nil, want non-nil")
		}
		if result.RequeueAfter != 0 {
			t.Fatalf("unexpected result from handleError(): %+v", result)
		}

		updated := &mintv1alpha1.CashuMint{}
		if getErr := client.Get(ctx, ctrlclient.ObjectKeyFromObject(mintCopy), updated); getErr != nil {
			t.Fatalf("Get() error = %v", getErr)
		}
		if updated.Status.Phase != mintv1alpha1.MintPhaseFailed {
			t.Fatalf("Phase = %q, want %q", updated.Status.Phase, mintv1alpha1.MintPhaseFailed)
		}
		condition := findCondition(updated.Status.Conditions, mintv1alpha1.ConditionTypeReady)
		if condition == nil || condition.Reason != "ReconciliationFailed" || condition.Message != "boom" {
			t.Fatalf("unexpected ready condition after failure: %+v", condition)
		}
	})
}

func TestHandleDeletionRemovesAutoProvisionedSecret(t *testing.T) {
	ctx := context.Background()
	mint := unitTestCashuMint("delete-me")
	mint.Finalizers = []string{cashuMintFinalizer}
	mint.Spec.Database = mintv1alpha1.DatabaseConfig{
		Engine: mintv1alpha1.DatabaseEnginePostgres,
		Postgres: &mintv1alpha1.PostgresConfig{
			AutoProvision: true,
		},
	}
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "delete-me-postgres-secret", Namespace: mint.Namespace}}

	reconciler, client := newUnitTestReconciler(t, mint, secret)
	if _, err := reconciler.handleDeletion(ctx, mint); err != nil {
		t.Fatalf("handleDeletion() error = %v", err)
	}

	updated := &mintv1alpha1.CashuMint{}
	if err := client.Get(ctx, ctrlclient.ObjectKeyFromObject(mint), updated); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if len(updated.Finalizers) != 0 {
		t.Fatalf("Finalizers = %v, want none", updated.Finalizers)
	}
	if err := client.Get(ctx, ctrlclient.ObjectKeyFromObject(secret), &corev1.Secret{}); !apierrors.IsNotFound(err) {
		t.Fatalf("expected auto-provisioned secret to be deleted, got %v", err)
	}
}

func TestUpdateStatus(t *testing.T) {
	ctx := context.Background()

	t.Run("records resource names and ready ingress URL", func(t *testing.T) {
		mint := unitTestCashuMint("status-ready")
		mint.Generation = 3
		mint.Spec.Ingress = &mintv1alpha1.IngressConfig{
			Enabled: true,
			Host:    "mint.example.com",
			TLS:     &mintv1alpha1.IngressTLSConfig{Enabled: true},
		}
		desired := mint.DeepCopy()
		desired.Status.Phase = mintv1alpha1.MintPhaseReady

		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: mint.Name, Namespace: mint.Namespace},
			Status:     appsv1.DeploymentStatus{ReadyReplicas: 2},
		}
		service := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: mint.Name, Namespace: mint.Namespace}}
		ingress := &networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{Name: mint.Name, Namespace: mint.Namespace},
			Spec:       networkingv1.IngressSpec{TLS: []networkingv1.IngressTLS{{Hosts: []string{"mint.example.com"}}}},
			Status: networkingv1.IngressStatus{
				LoadBalancer: networkingv1.IngressLoadBalancerStatus{
					Ingress: []networkingv1.IngressLoadBalancerIngress{{Hostname: "lb.example.com"}},
				},
			},
		}
		configMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: mint.Name + "-config", Namespace: mint.Namespace}}

		reconciler, client := newUnitTestReconciler(t, mint, deployment, service, ingress, configMap)
		if err := reconciler.updateStatus(ctx, desired); err != nil {
			t.Fatalf("updateStatus() error = %v", err)
		}

		updated := &mintv1alpha1.CashuMint{}
		if err := client.Get(ctx, ctrlclient.ObjectKeyFromObject(mint), updated); err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if updated.Status.ReadyReplicas != 2 {
			t.Fatalf("ReadyReplicas = %d, want 2", updated.Status.ReadyReplicas)
		}
		if updated.Status.DeploymentName != mint.Name || updated.Status.ServiceName != mint.Name || updated.Status.IngressName != mint.Name {
			t.Fatalf("unexpected managed resource names: %+v", updated.Status)
		}
		if updated.Status.ConfigMapName != mint.Name+"-config" {
			t.Fatalf("ConfigMapName = %q, want %q", updated.Status.ConfigMapName, mint.Name+"-config")
		}
		if updated.Status.URL != "https://mint.example.com" {
			t.Fatalf("URL = %q, want %q", updated.Status.URL, "https://mint.example.com")
		}
		if updated.Status.ObservedGeneration != mint.Generation {
			t.Fatalf("ObservedGeneration = %d, want %d", updated.Status.ObservedGeneration, mint.Generation)
		}
		condition := findCondition(updated.Status.Conditions, mintv1alpha1.ConditionTypeIngressReady)
		if condition == nil || condition.Status != metav1.ConditionTrue || condition.Reason != "IngressReady" {
			t.Fatalf("unexpected ingress condition: %+v", condition)
		}
	})

	t.Run("marks ingress not ready while waiting for an address", func(t *testing.T) {
		mint := unitTestCashuMint("status-pending")
		mint.Spec.Ingress = &mintv1alpha1.IngressConfig{Enabled: true, Host: "mint.example.com"}
		desired := mint.DeepCopy()

		ingress := &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: mint.Name, Namespace: mint.Namespace}}
		reconciler, client := newUnitTestReconciler(t, mint, ingress)
		if err := reconciler.updateStatus(ctx, desired); err != nil {
			t.Fatalf("updateStatus() error = %v", err)
		}

		updated := &mintv1alpha1.CashuMint{}
		if err := client.Get(ctx, ctrlclient.ObjectKeyFromObject(mint), updated); err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		condition := findCondition(updated.Status.Conditions, mintv1alpha1.ConditionTypeIngressReady)
		if condition == nil || condition.Status != metav1.ConditionFalse || condition.Reason != "IngressNotReady" {
			t.Fatalf("unexpected ingress condition: %+v", condition)
		}
	})
}

func TestReconcileConfigMapRequiresReadyPostgresSecret(t *testing.T) {
	ctx := context.Background()

	t.Run("returns a not-ready error when the secret is missing", func(t *testing.T) {
		mint := unitTestCashuMint("missing-secret")
		mint.Spec.Database = mintv1alpha1.DatabaseConfig{
			Engine: mintv1alpha1.DatabaseEnginePostgres,
			Postgres: &mintv1alpha1.PostgresConfig{
				AutoProvision: true,
			},
		}

		reconciler, _ := newUnitTestReconciler(t, mint)
		err := reconciler.reconcileConfigMap(ctx, mint)
		if err == nil || !strings.Contains(err.Error(), "postgres secret missing-secret-postgres-secret not ready yet") {
			t.Fatalf("reconcileConfigMap() error = %v", err)
		}
	})

	t.Run("returns an error when the password key is empty", func(t *testing.T) {
		mint := unitTestCashuMint("empty-secret")
		mint.Spec.Database = mintv1alpha1.DatabaseConfig{
			Engine: mintv1alpha1.DatabaseEnginePostgres,
			Postgres: &mintv1alpha1.PostgresConfig{
				AutoProvision: true,
			},
		}
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "empty-secret-postgres-secret", Namespace: mint.Namespace},
			Data:       map[string][]byte{"password": {}},
		}

		reconciler, _ := newUnitTestReconciler(t, mint, secret)
		err := reconciler.reconcileConfigMap(ctx, mint)
		if err == nil || !strings.Contains(err.Error(), "password key is empty") {
			t.Fatalf("reconcileConfigMap() error = %v", err)
		}
	})
}

func TestReconcileCertificateCreatesCertificate(t *testing.T) {
	ctx := context.Background()
	mint := unitTestCashuMint("certificate")
	mint.Spec.Ingress = &mintv1alpha1.IngressConfig{
		Enabled: true,
		Host:    "mint.example.com",
		Annotations: map[string]string{
			"example.com/issuer": "copied-to-cert",
		},
		TLS: &mintv1alpha1.IngressTLSConfig{
			Enabled:    true,
			SecretName: "certificate-tls",
			CertManager: &mintv1alpha1.CertManagerConfig{
				Enabled:    true,
				IssuerName: "letsencrypt",
				IssuerKind: "ClusterIssuer",
			},
		},
	}

	reconciler, client := newUnitTestReconciler(t, mint)
	if err := reconciler.reconcileCertificate(ctx, mint); err != nil {
		t.Fatalf("reconcileCertificate() error = %v", err)
	}

	certificate := &certmanagerv1.Certificate{}
	if err := client.Get(ctx, ctrlclient.ObjectKey{Name: mint.Name, Namespace: mint.Namespace}, certificate); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if certificate.Spec.SecretName != "certificate-tls" {
		t.Fatalf("SecretName = %q, want %q", certificate.Spec.SecretName, "certificate-tls")
	}
	if got, want := certificate.Spec.IssuerRef.Name, "letsencrypt"; got != want {
		t.Fatalf("IssuerRef.Name = %q, want %q", got, want)
	}
}

func TestReconcilePodMonitor(t *testing.T) {
	ctx := context.Background()

	t.Run("creates PodMonitor when metrics are enabled", func(t *testing.T) {
		mint := unitTestCashuMint("metrics-enabled")
		mint.Spec.Prometheus = &mintv1alpha1.PrometheusConfig{Enabled: true}

		reconciler, client := newUnitTestReconciler(t, mint)
		if err := reconciler.reconcilePodMonitor(ctx, mint); err != nil {
			t.Fatalf("reconcilePodMonitor() error = %v", err)
		}

		podMonitor := &monitoringv1.PodMonitor{}
		if err := client.Get(ctx, ctrlclient.ObjectKey{Name: mint.Name, Namespace: mint.Namespace}, podMonitor); err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if len(podMonitor.Spec.PodMetricsEndpoints) != 1 {
			t.Fatalf("endpoint count = %d, want 1", len(podMonitor.Spec.PodMetricsEndpoints))
		}
		if podMonitor.Spec.PodMetricsEndpoints[0].Port == nil || *podMonitor.Spec.PodMetricsEndpoints[0].Port != "metrics" {
			t.Fatalf("endpoint port = %v, want metrics", podMonitor.Spec.PodMetricsEndpoints[0].Port)
		}
	})

	t.Run("deletes PodMonitor when metrics are disabled", func(t *testing.T) {
		mint := unitTestCashuMint("metrics-disabled")
		existing := &monitoringv1.PodMonitor{
			ObjectMeta: metav1.ObjectMeta{Name: mint.Name, Namespace: mint.Namespace},
		}

		reconciler, client := newUnitTestReconciler(t, mint, existing)
		if err := reconciler.reconcilePodMonitor(ctx, mint); err != nil {
			t.Fatalf("reconcilePodMonitor() error = %v", err)
		}

		err := client.Get(ctx, ctrlclient.ObjectKey{Name: mint.Name, Namespace: mint.Namespace}, &monitoringv1.PodMonitor{})
		if !apierrors.IsNotFound(err) {
			t.Fatalf("expected PodMonitor to be deleted, got %v", err)
		}
	})

	t.Run("no-op when metrics are disabled and no PodMonitor exists", func(t *testing.T) {
		mint := unitTestCashuMint("metrics-disabled-no-existing")
		// Prometheus is nil — no PodMonitor pre-exists either.
		reconciler, _ := newUnitTestReconciler(t, mint)
		if err := reconciler.reconcilePodMonitor(ctx, mint); err != nil {
			t.Fatalf("reconcilePodMonitor() error = %v", err)
		}
	})

	t.Run("no-op when metrics are explicitly disabled via Enabled=false", func(t *testing.T) {
		mint := unitTestCashuMint("metrics-explicitly-disabled")
		mint.Spec.Prometheus = &mintv1alpha1.PrometheusConfig{Enabled: false}
		reconciler, _ := newUnitTestReconciler(t, mint)
		if err := reconciler.reconcilePodMonitor(ctx, mint); err != nil {
			t.Fatalf("reconcilePodMonitor() error = %v", err)
		}
	})

	t.Run("sets correct labels on PodMonitor", func(t *testing.T) {
		mint := unitTestCashuMint("metrics-labels")
		mint.Spec.Prometheus = &mintv1alpha1.PrometheusConfig{Enabled: true}

		reconciler, c := newUnitTestReconciler(t, mint)
		if err := reconciler.reconcilePodMonitor(ctx, mint); err != nil {
			t.Fatalf("reconcilePodMonitor() error = %v", err)
		}

		podMonitor := &monitoringv1.PodMonitor{}
		if err := c.Get(ctx, ctrlclient.ObjectKey{Name: mint.Name, Namespace: mint.Namespace}, podMonitor); err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		wantLabels := map[string]string{
			"app.kubernetes.io/name":       "cashu-mint",
			"app.kubernetes.io/instance":   mint.Name,
			"app.kubernetes.io/managed-by": "cashu-operator",
		}
		for k, want := range wantLabels {
			if got := podMonitor.Labels[k]; got != want {
				t.Errorf("label %q = %q, want %q", k, got, want)
			}
		}
	})

	t.Run("sets owner reference on PodMonitor", func(t *testing.T) {
		mint := unitTestCashuMint("metrics-ownerref")
		mint.UID = "test-uid-123"
		mint.Spec.Prometheus = &mintv1alpha1.PrometheusConfig{Enabled: true}

		reconciler, c := newUnitTestReconciler(t, mint)
		if err := reconciler.reconcilePodMonitor(ctx, mint); err != nil {
			t.Fatalf("reconcilePodMonitor() error = %v", err)
		}

		podMonitor := &monitoringv1.PodMonitor{}
		if err := c.Get(ctx, ctrlclient.ObjectKey{Name: mint.Name, Namespace: mint.Namespace}, podMonitor); err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if len(podMonitor.OwnerReferences) == 0 {
			t.Fatal("expected owner reference to be set on PodMonitor")
		}
		if podMonitor.OwnerReferences[0].Name != mint.Name {
			t.Errorf("owner reference name = %q, want %q", podMonitor.OwnerReferences[0].Name, mint.Name)
		}
	})
}

func TestReconcilePodMonitor_ErrorPaths(t *testing.T) {
	ctx := context.Background()

	t.Run("returns error when delete fails with unexpected error", func(t *testing.T) {
		mint := unitTestCashuMint("pm-delete-fail")
		// Prometheus is nil so reconcilePodMonitor will attempt a delete.
		existing := &monitoringv1.PodMonitor{
			ObjectMeta: metav1.ObjectMeta{Name: mint.Name, Namespace: mint.Namespace},
		}
		deleteErr := errors.New("unexpected delete failure")
		reconciler, _ := newUnitTestReconcilerWithInterceptor(t, interceptor.Funcs{
			Delete: func(ctx context.Context, c ctrlclient.WithWatch, obj ctrlclient.Object, opts ...ctrlclient.DeleteOption) error {
				if _, ok := obj.(*monitoringv1.PodMonitor); ok {
					return deleteErr
				}
				return c.Delete(ctx, obj, opts...)
			},
		}, mint, existing)

		err := reconciler.reconcilePodMonitor(ctx, mint)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, deleteErr) {
			t.Errorf("error = %v, want to wrap %v", err, deleteErr)
		}
	})

	t.Run("returns error when applyResource (Patch) fails", func(t *testing.T) {
		mint := unitTestCashuMint("pm-apply-fail")
		mint.Spec.Prometheus = &mintv1alpha1.PrometheusConfig{Enabled: true}
		patchErr := errors.New("patch failed")
		reconciler, _ := newUnitTestReconcilerWithInterceptor(t, interceptor.Funcs{
			Patch: func(ctx context.Context, c ctrlclient.WithWatch, obj ctrlclient.Object, patch ctrlclient.Patch, opts ...ctrlclient.PatchOption) error {
				if _, ok := obj.(*monitoringv1.PodMonitor); ok {
					return patchErr
				}
				return c.Patch(ctx, obj, patch, opts...)
			},
		}, mint)

		err := reconciler.reconcilePodMonitor(ctx, mint)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, patchErr) {
			t.Errorf("error = %v, want to wrap %v", err, patchErr)
		}
	})
}

func TestReconcileCoreResources_ErrorPaths(t *testing.T) {
	ctx := context.Background()

	t.Run("propagates reconcileDeployment error", func(t *testing.T) {
		mint := unitTestCashuMint("core-deploy-fail")
		patchErr := errors.New("deployment patch failed")
		reconciler, _ := newUnitTestReconcilerWithInterceptor(t, interceptor.Funcs{
			Patch: func(ctx context.Context, c ctrlclient.WithWatch, obj ctrlclient.Object, patch ctrlclient.Patch, opts ...ctrlclient.PatchOption) error {
				if _, ok := obj.(*appsv1.Deployment); ok {
					return patchErr
				}
				return c.Patch(ctx, obj, patch, opts...)
			},
		}, mint)

		err := reconciler.reconcileCoreResources(ctx, mint)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, patchErr) {
			t.Errorf("error = %v, want to wrap %v", err, patchErr)
		}
	})

	t.Run("propagates reconcileService error", func(t *testing.T) {
		mint := unitTestCashuMint("core-service-fail")
		patchErr := errors.New("service patch failed")
		reconciler, _ := newUnitTestReconcilerWithInterceptor(t, interceptor.Funcs{
			Patch: func(ctx context.Context, c ctrlclient.WithWatch, obj ctrlclient.Object, patch ctrlclient.Patch, opts ...ctrlclient.PatchOption) error {
				if _, ok := obj.(*corev1.Service); ok {
					return patchErr
				}
				return c.Patch(ctx, obj, patch, opts...)
			},
		}, mint)

		err := reconciler.reconcileCoreResources(ctx, mint)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, patchErr) {
			t.Errorf("error = %v, want to wrap %v", err, patchErr)
		}
	})

	t.Run("propagates reconcilePodMonitor error", func(t *testing.T) {
		mint := unitTestCashuMint("core-pm-fail")
		mint.Spec.Prometheus = &mintv1alpha1.PrometheusConfig{Enabled: true}
		patchErr := errors.New("podmonitor patch failed")
		reconciler, _ := newUnitTestReconcilerWithInterceptor(t, interceptor.Funcs{
			Patch: func(ctx context.Context, c ctrlclient.WithWatch, obj ctrlclient.Object, patch ctrlclient.Patch, opts ...ctrlclient.PatchOption) error {
				if _, ok := obj.(*monitoringv1.PodMonitor); ok {
					return patchErr
				}
				return c.Patch(ctx, obj, patch, opts...)
			},
		}, mint)

		err := reconciler.reconcileCoreResources(ctx, mint)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, patchErr) {
			t.Errorf("error = %v, want to wrap %v", err, patchErr)
		}
	})

	t.Run("propagates reconcileManagementRPCTLSSecret error via Create", func(t *testing.T) {
		mint := unitTestCashuMint("core-mgmt-tls-fail")
		mint.Spec.ManagementRPC = &mintv1alpha1.ManagementRPCConfig{
			Enabled:      true,
			TLSSecretRef: &corev1.LocalObjectReference{Name: "mgmt-tls"},
		}
		createErr := errors.New("create tls secret failed")
		reconciler, _ := newUnitTestReconcilerWithInterceptor(t, interceptor.Funcs{
			Create: func(ctx context.Context, c ctrlclient.WithWatch, obj ctrlclient.Object, opts ...ctrlclient.CreateOption) error {
				if _, ok := obj.(*corev1.Secret); ok {
					return createErr
				}
				return c.Create(ctx, obj, opts...)
			},
		}, mint)

		err := reconciler.reconcileCoreResources(ctx, mint)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, createErr) {
			t.Errorf("error = %v, want to wrap %v", err, createErr)
		}
	})

	t.Run("propagates reconcileConfigMap error via Create", func(t *testing.T) {
		mint := unitTestCashuMint("core-configmap-fail")
		createErr := errors.New("create configmap failed")
		patchCallCount := 0
		reconciler, _ := newUnitTestReconcilerWithInterceptor(t, interceptor.Funcs{
			Patch: func(ctx context.Context, c ctrlclient.WithWatch, obj ctrlclient.Object, patch ctrlclient.Patch, opts ...ctrlclient.PatchOption) error {
				if _, ok := obj.(*corev1.ConfigMap); ok {
					patchCallCount++
					return createErr
				}
				return c.Patch(ctx, obj, patch, opts...)
			},
		}, mint)

		err := reconciler.reconcileCoreResources(ctx, mint)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, createErr) {
			t.Errorf("error = %v, want to wrap %v", err, createErr)
		}
	})

	t.Run("propagates reconcilePVC error", func(t *testing.T) {
		mint := unitTestCashuMint("core-pvc-fail")
		// SQLite requires PVC reconciliation
		patchErr := errors.New("pvc patch failed")
		reconciler, _ := newUnitTestReconcilerWithInterceptor(t, interceptor.Funcs{
			Patch: func(ctx context.Context, c ctrlclient.WithWatch, obj ctrlclient.Object, patch ctrlclient.Patch, opts ...ctrlclient.PatchOption) error {
				if _, ok := obj.(*corev1.PersistentVolumeClaim); ok {
					return patchErr
				}
				return c.Patch(ctx, obj, patch, opts...)
			},
		}, mint)

		err := reconciler.reconcileCoreResources(ctx, mint)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, patchErr) {
			t.Errorf("error = %v, want to wrap %v", err, patchErr)
		}
	})
}

func TestReconcilePodMonitor_NoMatchError(t *testing.T) {
	ctx := context.Background()

	t.Run("returns wrapped error when apply returns NoKindMatchError (CRD not installed)", func(t *testing.T) {
		mint := unitTestCashuMint("pm-no-match")
		mint.Spec.Prometheus = &mintv1alpha1.PrometheusConfig{Enabled: true}
		noMatchErr := &apimeta.NoKindMatchError{GroupKind: schema.GroupKind{Group: "monitoring.coreos.com", Kind: "PodMonitor"}}
		reconciler, _ := newUnitTestReconcilerWithInterceptor(t, interceptor.Funcs{
			Patch: func(ctx context.Context, c ctrlclient.WithWatch, obj ctrlclient.Object, patch ctrlclient.Patch, opts ...ctrlclient.PatchOption) error {
				if _, ok := obj.(*monitoringv1.PodMonitor); ok {
					return noMatchErr
				}
				return c.Patch(ctx, obj, patch, opts...)
			},
		}, mint)

		err := reconciler.reconcilePodMonitor(ctx, mint)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, noMatchErr) {
			t.Errorf("error = %v, want to wrap %v", err, noMatchErr)
		}
	})

	t.Run("no-op when delete returns NoKindMatchError (CRD not installed)", func(t *testing.T) {
		mint := unitTestCashuMint("pm-delete-no-match")
		// Prometheus nil: will attempt delete, CRD not installed should be treated as no-op.
		noMatchErr := &apimeta.NoKindMatchError{GroupKind: schema.GroupKind{Group: "monitoring.coreos.com", Kind: "PodMonitor"}}
		reconciler, _ := newUnitTestReconcilerWithInterceptor(t, interceptor.Funcs{
			Delete: func(ctx context.Context, c ctrlclient.WithWatch, obj ctrlclient.Object, opts ...ctrlclient.DeleteOption) error {
				if _, ok := obj.(*monitoringv1.PodMonitor); ok {
					return noMatchErr
				}
				return c.Delete(ctx, obj, opts...)
			},
		}, mint)

		if err := reconciler.reconcilePodMonitor(ctx, mint); err != nil {
			t.Fatalf("expected no error when CRD not installed, got %v", err)
		}
	})
}

func TestReconcileOptionalMnemonic(t *testing.T) {
	ctx := context.Background()

	t.Run("creates an auto-generated mnemonic secret", func(t *testing.T) {
		mint := unitTestCashuMint("auto-mnemonic")
		mint.Spec.MintInfo.AutoGenerateMnemonic = true

		reconciler, client := newUnitTestReconciler(t, mint)
		if err := reconciler.reconcileOptionalMnemonic(ctx, mint); err != nil {
			t.Fatalf("reconcileOptionalMnemonic() error = %v", err)
		}

		secret := &corev1.Secret{}
		if err := client.Get(ctx, ctrlclient.ObjectKey{Name: generators.MnemonicSecretName(mint.Name), Namespace: mint.Namespace}, secret); err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		mnemonic := string(secret.Data[generators.MnemonicSecretKey])
		if mnemonic == "" {
			mnemonic = secret.StringData[generators.MnemonicSecretKey]
		}
		if mnemonic == "" {
			t.Fatal("expected generated mnemonic in secret data")
		}
		if len(strings.Fields(mnemonic)) != 24 {
			t.Fatalf("generated mnemonic has %d words, want 24", len(strings.Fields(mnemonic)))
		}
	})

	t.Run("keeps an existing mnemonic secret", func(t *testing.T) {
		mint := unitTestCashuMint("existing-mnemonic")
		mint.Spec.MintInfo.AutoGenerateMnemonic = true
		existingMnemonic := "existing mnemonic phrase that should be preserved"
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      generators.MnemonicSecretName(mint.Name),
				Namespace: mint.Namespace,
			},
			Data: map[string][]byte{
				generators.MnemonicSecretKey: []byte(existingMnemonic),
			},
		}

		reconciler, client := newUnitTestReconciler(t, mint, secret)
		if err := reconciler.reconcileOptionalMnemonic(ctx, mint); err != nil {
			t.Fatalf("reconcileOptionalMnemonic() error = %v", err)
		}

		updated := &corev1.Secret{}
		if err := client.Get(ctx, ctrlclient.ObjectKeyFromObject(secret), updated); err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if got := string(updated.Data[generators.MnemonicSecretKey]); got != existingMnemonic {
			t.Fatalf("mnemonic = %q, want existing mnemonic", got)
		}
	})
}

func TestReconcileManagementRPCTLSSecret(t *testing.T) {
	ctx := context.Background()

	t.Run("creates the configured TLS secret when missing", func(t *testing.T) {
		mint := unitTestCashuMint("management-rpc-tls")
		mint.Spec.ManagementRPC = &mintv1alpha1.ManagementRPCConfig{
			Enabled:      true,
			TLSSecretRef: &corev1.LocalObjectReference{Name: "custom-management-rpc-tls"},
		}

		reconciler, client := newUnitTestReconciler(t, mint)
		if err := reconciler.reconcileManagementRPCTLSSecret(ctx, mint); err != nil {
			t.Fatalf("reconcileManagementRPCTLSSecret() error = %v", err)
		}

		secret := &corev1.Secret{}
		if err := client.Get(ctx, ctrlclient.ObjectKey{Name: "custom-management-rpc-tls", Namespace: mint.Namespace}, secret); err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		for _, key := range []string{"ca.pem", "server.pem", "server.key", "client.pem", "client.key"} {
			if _, ok := secret.Data[key]; !ok {
				t.Fatalf("expected TLS secret to contain %q", key)
			}
		}
	})

	t.Run("does not overwrite an existing TLS secret", func(t *testing.T) {
		mint := unitTestCashuMint("management-rpc-existing")
		mint.Spec.ManagementRPC = &mintv1alpha1.ManagementRPCConfig{
			Enabled:      true,
			TLSSecretRef: &corev1.LocalObjectReference{Name: "existing-management-rpc-tls"},
		}
		existingSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "existing-management-rpc-tls",
				Namespace: mint.Namespace,
			},
			Data: map[string][]byte{
				"ca.pem": []byte("custom-ca"),
			},
		}

		reconciler, client := newUnitTestReconciler(t, mint, existingSecret)
		if err := reconciler.reconcileManagementRPCTLSSecret(ctx, mint); err != nil {
			t.Fatalf("reconcileManagementRPCTLSSecret() error = %v", err)
		}

		updated := &corev1.Secret{}
		if err := client.Get(ctx, ctrlclient.ObjectKeyFromObject(existingSecret), updated); err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if got := string(updated.Data["ca.pem"]); got != "custom-ca" {
			t.Fatalf("ca.pem = %q, want existing data to be preserved", got)
		}
		if _, ok := updated.Data["server.pem"]; ok {
			t.Fatalf("expected existing secret not to be regenerated, got server.pem in %+v", updated.Data)
		}
	})
}

func newUnitTestReconciler(t *testing.T, objects ...ctrlclient.Object) (*CashuMintReconciler, ctrlclient.Client) {
	t.Helper()

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(mintv1alpha1.AddToScheme(scheme))
	utilruntime.Must(certmanagerv1.AddToScheme(scheme))
	utilruntime.Must(monitoringv1.AddToScheme(scheme))

	builder := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&mintv1alpha1.CashuMint{})
	if len(objects) > 0 {
		builder = builder.WithObjects(objects...)
	}
	client := builder.Build()

	return &CashuMintReconciler{
		Client:   client,
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(16),
	}, client
}

func unitTestCashuMint(name string) *mintv1alpha1.CashuMint {
	return &mintv1alpha1.CashuMint{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Namespace:  "default",
			Generation: 1,
		},
		Spec: mintv1alpha1.CashuMintSpec{
			MintInfo: mintv1alpha1.MintInfo{URL: "https://mint.example.com"},
			Database: mintv1alpha1.DatabaseConfig{
				Engine: mintv1alpha1.DatabaseEngineSQLite,
				SQLite: &mintv1alpha1.SQLiteConfig{DataDir: "/data"},
			},
			PaymentBackend: mintv1alpha1.PaymentBackendConfig{
				FakeWallet: &mintv1alpha1.FakeWalletConfig{},
			},
		},
	}
}

func newUnitTestReconcilerWithInterceptor(t *testing.T, funcs interceptor.Funcs, objects ...ctrlclient.Object) (*CashuMintReconciler, ctrlclient.Client) {
	t.Helper()

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(mintv1alpha1.AddToScheme(scheme))
	utilruntime.Must(certmanagerv1.AddToScheme(scheme))
	utilruntime.Must(monitoringv1.AddToScheme(scheme))

	builder := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&mintv1alpha1.CashuMint{}).WithInterceptorFuncs(funcs)
	if len(objects) > 0 {
		builder = builder.WithObjects(objects...)
	}
	client := builder.Build()

	return &CashuMintReconciler{
		Client:   client,
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(16),
	}, client
}

func resourceForTests() schema.GroupResource {
	return schema.GroupResource{Group: "mint.cashu.asmogo.github.io", Resource: "cashumints"}
}

func findCondition(conditions []metav1.Condition, conditionType string) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}
