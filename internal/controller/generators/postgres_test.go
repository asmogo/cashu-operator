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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	mintv1alpha1 "github.com/asmogo/cashu-operator/api/v1alpha1"
)

func TestGeneratePostgresSecret_AutoProvision(t *testing.T) {
	mint := &mintv1alpha1.CashuMint{
		ObjectMeta: metav1.ObjectMeta{Name: "pg-mint", Namespace: "default"},
		Spec: mintv1alpha1.CashuMintSpec{
			Database: mintv1alpha1.DatabaseConfig{
				Engine:   "postgres",
				Postgres: &mintv1alpha1.PostgresConfig{AutoProvision: true},
			},
		},
	}

	secret, err := GeneratePostgresSecret(mint, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if secret == nil {
		t.Fatal("expected Secret, got nil")
	}
	if secret.Name != "pg-mint-postgres-secret" {
		t.Errorf("name = %q, want %q", secret.Name, "pg-mint-postgres-secret")
	}
	if secret.StringData["password"] == "" {
		t.Error("password should be generated")
	}
	if secret.StringData["database-url"] == "" {
		t.Error("database-url should be set")
	}
	assertContains(t, secret.StringData["database-url"], "postgresql://cdk:")
	assertContains(t, secret.StringData["database-url"], "@pg-mint-postgres:5432/cdk_mintd")
}

func TestGeneratePostgresSecret_ExistingPassword(t *testing.T) {
	mint := &mintv1alpha1.CashuMint{
		ObjectMeta: metav1.ObjectMeta{Name: "pg-mint", Namespace: "default"},
		Spec: mintv1alpha1.CashuMintSpec{
			Database: mintv1alpha1.DatabaseConfig{
				Engine:   "postgres",
				Postgres: &mintv1alpha1.PostgresConfig{AutoProvision: true},
			},
		},
	}

	secret, err := GeneratePostgresSecret(mint, "existing-pw")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if secret.StringData["password"] != "existing-pw" {
		t.Errorf("password = %q, want %q", secret.StringData["password"], "existing-pw")
	}
}

func TestGeneratePostgresSecret_NoAutoProvision(t *testing.T) {
	mint := &mintv1alpha1.CashuMint{
		ObjectMeta: metav1.ObjectMeta{Name: "ext-mint", Namespace: "default"},
		Spec: mintv1alpha1.CashuMintSpec{
			Database: mintv1alpha1.DatabaseConfig{
				Engine:   "postgres",
				Postgres: &mintv1alpha1.PostgresConfig{AutoProvision: false},
			},
		},
	}

	secret, err := GeneratePostgresSecret(mint, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if secret != nil {
		t.Errorf("expected nil for non-auto-provisioned, got %v", secret)
	}
}

func TestGeneratePostgresSecret_NilPostgres(t *testing.T) {
	mint := &mintv1alpha1.CashuMint{
		ObjectMeta: metav1.ObjectMeta{Name: "no-pg", Namespace: "default"},
		Spec: mintv1alpha1.CashuMintSpec{
			Database: mintv1alpha1.DatabaseConfig{Engine: "sqlite"},
		},
	}

	secret, err := GeneratePostgresSecret(mint, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if secret != nil {
		t.Errorf("expected nil for sqlite engine, got %v", secret)
	}
}

func TestGeneratePostgresService_AutoProvision(t *testing.T) {
	scheme := testScheme(t)
	mint := &mintv1alpha1.CashuMint{
		ObjectMeta: metav1.ObjectMeta{Name: "pg-mint", Namespace: "default"},
		Spec: mintv1alpha1.CashuMintSpec{
			Database: mintv1alpha1.DatabaseConfig{
				Engine:   "postgres",
				Postgres: &mintv1alpha1.PostgresConfig{AutoProvision: true},
			},
		},
	}

	svc, err := GeneratePostgresService(mint, scheme)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc == nil {
		t.Fatal("expected Service, got nil")
	}
	if svc.Name != "pg-mint-postgres" {
		t.Errorf("name = %q, want %q", svc.Name, "pg-mint-postgres")
	}
	if svc.Spec.ClusterIP != "None" {
		t.Errorf("clusterIP = %q, want None (headless)", svc.Spec.ClusterIP)
	}
	if len(svc.Spec.Ports) != 1 || svc.Spec.Ports[0].Port != 5432 {
		t.Errorf("expected port 5432")
	}
}

func TestGeneratePostgresService_NoAutoProvision(t *testing.T) {
	scheme := testScheme(t)
	mint := &mintv1alpha1.CashuMint{
		ObjectMeta: metav1.ObjectMeta{Name: "ext-mint", Namespace: "default"},
		Spec: mintv1alpha1.CashuMintSpec{
			Database: mintv1alpha1.DatabaseConfig{
				Engine:   "postgres",
				Postgres: &mintv1alpha1.PostgresConfig{AutoProvision: false},
			},
		},
	}

	svc, err := GeneratePostgresService(mint, scheme)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc != nil {
		t.Errorf("expected nil, got %v", svc)
	}
}

func TestGeneratePostgresStatefulSet_Defaults(t *testing.T) {
	scheme := testScheme(t)
	mint := &mintv1alpha1.CashuMint{
		ObjectMeta: metav1.ObjectMeta{Name: "pg-mint", Namespace: "default"},
		Spec: mintv1alpha1.CashuMintSpec{
			Database: mintv1alpha1.DatabaseConfig{
				Engine:   "postgres",
				Postgres: &mintv1alpha1.PostgresConfig{AutoProvision: true},
			},
		},
	}

	sts, err := GeneratePostgresStatefulSet(mint, scheme)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sts == nil {
		t.Fatal("expected StatefulSet, got nil")
	}
	if sts.Name != "pg-mint-postgres" {
		t.Errorf("name = %q, want %q", sts.Name, "pg-mint-postgres")
	}
	if *sts.Spec.Replicas != 1 {
		t.Errorf("replicas = %d, want 1", *sts.Spec.Replicas)
	}
	if sts.Spec.ServiceName != "pg-mint-postgres" {
		t.Errorf("serviceName = %q, want %q", sts.Spec.ServiceName, "pg-mint-postgres")
	}

	// Check default PVC size
	if len(sts.Spec.VolumeClaimTemplates) != 1 {
		t.Fatalf("VCT count = %d, want 1", len(sts.Spec.VolumeClaimTemplates))
	}
	expectedSize := resource.MustParse("10Gi")
	actualSize := sts.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage]
	if !actualSize.Equal(expectedSize) {
		t.Errorf("storage = %s, want %s", actualSize.String(), expectedSize.String())
	}

	// Check init container exists
	if len(sts.Spec.Template.Spec.InitContainers) != 1 {
		t.Fatalf("init containers = %d, want 1", len(sts.Spec.Template.Spec.InitContainers))
	}
	if sts.Spec.Template.Spec.InitContainers[0].Name != "init-password" {
		t.Errorf("init container name = %q, want %q", sts.Spec.Template.Spec.InitContainers[0].Name, "init-password")
	}

	// Check container image uses default version 15
	assertContains(t, sts.Spec.Template.Spec.Containers[0].Image, "postgres:15-alpine")
}

func TestGeneratePostgresStatefulSet_CustomSpec(t *testing.T) {
	scheme := testScheme(t)
	mint := &mintv1alpha1.CashuMint{
		ObjectMeta: metav1.ObjectMeta{Name: "custom-pg", Namespace: "ns1"},
		Spec: mintv1alpha1.CashuMintSpec{
			Database: mintv1alpha1.DatabaseConfig{
				Engine: "postgres",
				Postgres: &mintv1alpha1.PostgresConfig{
					AutoProvision: true,
					AutoProvisionSpec: &mintv1alpha1.PostgresAutoProvisionSpec{
						StorageSize:      "50Gi",
						Version:          "16",
						StorageClassName: stringPtr("fast-ssd"),
						Resources: &corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("1Gi"),
							},
						},
					},
				},
			},
		},
	}

	sts, err := GeneratePostgresStatefulSet(mint, scheme)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Custom version
	assertContains(t, sts.Spec.Template.Spec.Containers[0].Image, "postgres:16-alpine")

	// Custom storage
	expectedSize := resource.MustParse("50Gi")
	actualSize := sts.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage]
	if !actualSize.Equal(expectedSize) {
		t.Errorf("storage = %s, want %s", actualSize.String(), expectedSize.String())
	}

	// Custom storage class
	sc := sts.Spec.VolumeClaimTemplates[0].Spec.StorageClassName
	if sc == nil || *sc != "fast-ssd" {
		t.Errorf("storageClassName = %v, want fast-ssd", sc)
	}

	// Custom resources
	cpuReq := sts.Spec.Template.Spec.Containers[0].Resources.Requests[corev1.ResourceCPU]
	expectedCPU := resource.MustParse("500m")
	if !cpuReq.Equal(expectedCPU) {
		t.Errorf("cpu request = %s, want %s", cpuReq.String(), expectedCPU.String())
	}
}

func TestGeneratePostgresStatefulSet_NoAutoProvision(t *testing.T) {
	scheme := testScheme(t)
	mint := &mintv1alpha1.CashuMint{
		ObjectMeta: metav1.ObjectMeta{Name: "ext-pg", Namespace: "default"},
		Spec: mintv1alpha1.CashuMintSpec{
			Database: mintv1alpha1.DatabaseConfig{
				Engine:   "postgres",
				Postgres: &mintv1alpha1.PostgresConfig{AutoProvision: false},
			},
		},
	}

	sts, err := GeneratePostgresStatefulSet(mint, scheme)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sts != nil {
		t.Errorf("expected nil, got %v", sts)
	}
}

func TestGenerateSecurePassword(t *testing.T) {
	pw1, err := generateSecurePassword(32)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pw1) != 32 {
		t.Errorf("length = %d, want 32", len(pw1))
	}

	pw2, _ := generateSecurePassword(32)
	if pw1 == pw2 {
		t.Error("expected different passwords on separate calls")
	}
}
