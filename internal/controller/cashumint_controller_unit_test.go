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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	mintv1alpha1 "github.com/asmogo/cashu-operator/api/v1alpha1"
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
