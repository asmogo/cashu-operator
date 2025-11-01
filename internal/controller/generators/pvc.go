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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	mintv1alpha1 "github.com/asmogo/cashu-operator/api/v1alpha1"
)

// GeneratePVC creates a PersistentVolumeClaim for the Cashu mint data
func GeneratePVC(mint *mintv1alpha1.CashuMint, scheme *runtime.Scheme) (*corev1.PersistentVolumeClaim, error) {
	// Only create PVC for SQLite or redb engines
	if mint.Spec.Database.Engine != "sqlite" && mint.Spec.Database.Engine != "redb" {
		return nil, nil
	}

	labels := map[string]string{
		"app.kubernetes.io/name":       "cashu-mint",
		"app.kubernetes.io/instance":   mint.Name,
		"app.kubernetes.io/component":  "storage",
		"app.kubernetes.io/managed-by": "cashu-operator",
	}

	size := "10Gi"
	if mint.Spec.Storage != nil && mint.Spec.Storage.Size != "" {
		size = mint.Spec.Storage.Size
	}

	pvc := &corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "PersistentVolumeClaim",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      mint.Name + "-data",
			Namespace: mint.Namespace,
			Labels:    labels,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(size),
				},
			},
		},
	}

	// Add storage class if specified
	if mint.Spec.Storage != nil && mint.Spec.Storage.StorageClassName != nil {
		pvc.Spec.StorageClassName = mint.Spec.Storage.StorageClassName
	}

	if err := controllerutil.SetControllerReference(mint, pvc, scheme); err != nil {
		return nil, fmt.Errorf("failed to set controller reference: %w", err)
	}

	return pvc, nil
}
