/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package generators

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	mintv1alpha1 "github.com/asmogo/cashu-operator/api/v1alpha1"
)

func TestGeneratePodMonitor_Disabled(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("no-podmonitor")

	podMonitor, err := GeneratePodMonitor(mint, scheme)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if podMonitor != nil {
		t.Fatalf("expected nil PodMonitor, got %#v", podMonitor)
	}
}

func TestGeneratePodMonitor_Enabled(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("metrics-mint")
	mint.Spec.Prometheus = &mintv1alpha1.PrometheusConfig{Enabled: true}

	podMonitor, err := GeneratePodMonitor(mint, scheme)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if podMonitor == nil {
		t.Fatal("expected PodMonitor, got nil")
	}
	if podMonitor.Name != mint.Name {
		t.Fatalf("name = %q, want %q", podMonitor.Name, mint.Name)
	}
	if len(podMonitor.Spec.PodMetricsEndpoints) != 1 {
		t.Fatalf("endpoint count = %d, want 1", len(podMonitor.Spec.PodMetricsEndpoints))
	}

	endpoint := podMonitor.Spec.PodMetricsEndpoints[0]
	if endpoint.Port == nil || *endpoint.Port != "metrics" {
		t.Fatalf("endpoint port = %v, want metrics", endpoint.Port)
	}
	if endpoint.Path != "/metrics" {
		t.Fatalf("endpoint path = %q, want /metrics", endpoint.Path)
	}
	if got := podMonitor.Spec.Selector.MatchLabels["app.kubernetes.io/instance"]; got != mint.Name {
		t.Fatalf("selector instance label = %q, want %q", got, mint.Name)
	}
	if len(podMonitor.OwnerReferences) == 0 {
		t.Fatal("expected owner reference to be set")
	}
}

func TestGeneratePodMonitor_OwnerReference(t *testing.T) {
	scheme := testScheme(t)
	mint := &mintv1alpha1.CashuMint{
		ObjectMeta: metav1.ObjectMeta{Name: "ref-podmonitor", Namespace: "default", UID: "test-uid"},
		Spec: mintv1alpha1.CashuMintSpec{
			MintInfo:       mintv1alpha1.MintInfo{URL: "http://test.local"},
			Database:       mintv1alpha1.DatabaseConfig{Engine: "sqlite"},
			PaymentBackend: mintv1alpha1.PaymentBackendConfig{FakeWallet: &mintv1alpha1.FakeWalletConfig{}},
			Prometheus:     &mintv1alpha1.PrometheusConfig{Enabled: true},
		},
	}

	podMonitor, err := GeneratePodMonitor(mint, scheme)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(podMonitor.OwnerReferences) == 0 {
		t.Fatal("expected owner reference to be set")
	}
}

func TestGeneratePodMonitor_ExplicitlyDisabled(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("explicitly-disabled")
	mint.Spec.Prometheus = &mintv1alpha1.PrometheusConfig{Enabled: false}

	podMonitor, err := GeneratePodMonitor(mint, scheme)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if podMonitor != nil {
		t.Fatalf("expected nil PodMonitor when Enabled=false, got %#v", podMonitor)
	}
}

func TestGeneratePodMonitor_Labels(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("label-check")
	mint.Spec.Prometheus = &mintv1alpha1.PrometheusConfig{Enabled: true}

	podMonitor, err := GeneratePodMonitor(mint, scheme)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if podMonitor == nil {
		t.Fatal("expected PodMonitor, got nil")
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
		if got := podMonitor.Spec.Selector.MatchLabels[k]; got != want {
			t.Errorf("selector label %q = %q, want %q", k, got, want)
		}
	}
}

func TestGeneratePodMonitor_TypeMeta(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("typemeta-check")
	mint.Spec.Prometheus = &mintv1alpha1.PrometheusConfig{Enabled: true}

	podMonitor, err := GeneratePodMonitor(mint, scheme)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if podMonitor == nil {
		t.Fatal("expected PodMonitor, got nil")
	}
	if podMonitor.Kind != "PodMonitor" {
		t.Errorf("Kind = %q, want PodMonitor", podMonitor.Kind)
	}
	if podMonitor.APIVersion == "" {
		t.Error("APIVersion should not be empty")
	}
}
