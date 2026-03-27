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

func TestGeneratePVC(t *testing.T) {
	scheme := testScheme(t)

	t.Run("sqlite engine creates PVC with defaults", func(t *testing.T) {
		mint := &mintv1alpha1.CashuMint{
			ObjectMeta: metav1.ObjectMeta{Name: "my-mint", Namespace: "default"},
			Spec: mintv1alpha1.CashuMintSpec{
				Database: mintv1alpha1.DatabaseConfig{Engine: "sqlite"},
			},
		}

		pvc, err := GeneratePVC(mint, scheme)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if pvc == nil {
			t.Fatal("expected PVC, got nil")
		}
		if pvc.Name != "my-mint-data" {
			t.Errorf("name = %q, want %q", pvc.Name, "my-mint-data")
		}
		if pvc.Namespace != "default" {
			t.Errorf("namespace = %q, want %q", pvc.Namespace, "default")
		}

		expectedSize := resource.MustParse("10Gi")
		actualSize := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
		if !actualSize.Equal(expectedSize) {
			t.Errorf("storage size = %s, want %s", actualSize.String(), expectedSize.String())
		}

		if len(pvc.Spec.AccessModes) != 1 || pvc.Spec.AccessModes[0] != corev1.ReadWriteOnce {
			t.Errorf("access modes = %v, want [ReadWriteOnce]", pvc.Spec.AccessModes)
		}

		if pvc.Spec.StorageClassName != nil {
			t.Errorf("storage class should be nil, got %v", pvc.Spec.StorageClassName)
		}

		assertLabelsContain(t, pvc.Labels, "app.kubernetes.io/instance", "my-mint")
		assertLabelsContain(t, pvc.Labels, "app.kubernetes.io/component", "storage")
	})

	t.Run("redb engine creates PVC", func(t *testing.T) {
		mint := &mintv1alpha1.CashuMint{
			ObjectMeta: metav1.ObjectMeta{Name: "redb-mint", Namespace: "ns1"},
			Spec: mintv1alpha1.CashuMintSpec{
				Database: mintv1alpha1.DatabaseConfig{Engine: "redb"},
			},
		}

		pvc, err := GeneratePVC(mint, scheme)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if pvc == nil {
			t.Fatal("expected PVC for redb, got nil")
		}
		if pvc.Name != "redb-mint-data" {
			t.Errorf("name = %q, want %q", pvc.Name, "redb-mint-data")
		}
	})

	t.Run("postgres engine returns nil", func(t *testing.T) {
		mint := &mintv1alpha1.CashuMint{
			ObjectMeta: metav1.ObjectMeta{Name: "pg-mint", Namespace: "default"},
			Spec: mintv1alpha1.CashuMintSpec{
				Database: mintv1alpha1.DatabaseConfig{Engine: "postgres"},
			},
		}

		pvc, err := GeneratePVC(mint, scheme)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if pvc != nil {
			t.Errorf("expected nil PVC for postgres, got %v", pvc)
		}
	})

	t.Run("custom size and storage class", func(t *testing.T) {
		storageClass := "fast-ssd"
		mint := &mintv1alpha1.CashuMint{
			ObjectMeta: metav1.ObjectMeta{Name: "custom-mint", Namespace: "default"},
			Spec: mintv1alpha1.CashuMintSpec{
				Database: mintv1alpha1.DatabaseConfig{Engine: "sqlite"},
				Storage: &mintv1alpha1.StorageConfig{
					Size:             "50Gi",
					StorageClassName: &storageClass,
				},
			},
		}

		pvc, err := GeneratePVC(mint, scheme)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if pvc == nil {
			t.Fatal("expected PVC, got nil")
		}

		expectedSize := resource.MustParse("50Gi")
		actualSize := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
		if !actualSize.Equal(expectedSize) {
			t.Errorf("storage size = %s, want %s", actualSize.String(), expectedSize.String())
		}

		if pvc.Spec.StorageClassName == nil || *pvc.Spec.StorageClassName != "fast-ssd" {
			t.Errorf("storage class = %v, want %q", pvc.Spec.StorageClassName, "fast-ssd")
		}
	})
}

// assertLabelsContain checks that a label map contains a specific key-value pair.
func assertLabelsContain(t *testing.T, labels map[string]string, key, value string) {
	t.Helper()
	if labels[key] != value {
		t.Errorf("label %q = %q, want %q", key, labels[key], value)
	}
}
