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

package controller

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/asmogo/cashu-operator/internal/resources"
	"github.com/asmogo/cashu-operator/internal/status"
)

// calculateConfigHash calculates a SHA256 hash of the ConfigMap data.
// DEPRECATED: Use resources.ConfigMapHash instead.
func calculateConfigHash(configMap *corev1.ConfigMap) string {
	return resources.ConfigMapHash(configMap)
}

// ensureOwnerReference sets the owner reference on a dependent object
func ensureOwnerReference(owner, dependent metav1.Object, scheme *runtime.Scheme) error {
	ownerObj, ok := owner.(runtime.Object)
	if !ok {
		return fmt.Errorf("owner is not a runtime.Object")
	}
	dependentObj, ok := dependent.(runtime.Object)
	if !ok {
		return fmt.Errorf("dependent is not a runtime.Object")
	}
	return controllerutil.SetControllerReference(ownerObj.(client.Object), dependentObj.(client.Object), scheme)
}

// isDeploymentReady checks if a Deployment is ready.
// DEPRECATED: Use status.IsDeploymentReady instead.
func isDeploymentReady(deployment *appsv1.Deployment) bool {
	return status.IsDeploymentReady(deployment)
}

// isStatefulSetReady checks if a StatefulSet is ready.
// DEPRECATED: Use status.IsStatefulSetReady instead.
func isStatefulSetReady(sts *appsv1.StatefulSet) bool {
	return status.IsStatefulSetReady(sts)
}

// isServiceReady checks if a Service is ready.
// DEPRECATED: Use status.IsServiceReady instead.
func isServiceReady(service *corev1.Service) bool {
	return status.IsServiceReady(service)
}

// isIngressReady checks if an Ingress is ready.
// DEPRECATED: Use status.IsIngressReady instead.
func isIngressReady(ingress *networkingv1.Ingress) bool {
	return status.IsIngressReady(ingress)
}

// isPVCBound checks if a PVC is bound.
// DEPRECATED: Use status.IsPVCBound instead.
func isPVCBound(pvc *corev1.PersistentVolumeClaim) bool {
	return status.IsPVCBound(pvc)
}

// waitForPostgreSQLReady waits for PostgreSQL StatefulSet to be ready
func waitForPostgreSQLReady(ctx context.Context, c client.Client, name, namespace string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for PostgreSQL to be ready")
		case <-ticker.C:
			sts := &appsv1.StatefulSet{}
			err := c.Get(ctx, types.NamespacedName{
				Name:      name + "-postgres",
				Namespace: namespace,
			}, sts)

			if err != nil {
				if apierrors.IsNotFound(err) {
					continue
				}
				return fmt.Errorf("failed to get PostgreSQL StatefulSet: %w", err)
			}

			if isStatefulSetReady(sts) {
				return nil
			}
		}
	}
}

// getResourceOrNil gets a resource and returns nil if not found
func getResourceOrNil(ctx context.Context, c client.Client, key types.NamespacedName, obj client.Object) error {
	err := c.Get(ctx, key, obj)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return nil
}

// applyResource creates or updates a resource using server-side apply
func applyResource(ctx context.Context, c client.Client, obj client.Object) error {
	return c.Patch(ctx, obj, client.Apply, client.ForceOwnership, client.FieldOwner("cashu-operator"))
}

// deleteResourceIfExists deletes a resource if it exists
func deleteResourceIfExists(ctx context.Context, c client.Client, obj client.Object) error {
	err := c.Delete(ctx, obj)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}
