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
	"crypto/sha256"
	"encoding/hex"
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
)

// calculateConfigHash calculates a SHA256 hash of the ConfigMap data
func calculateConfigHash(configMap *corev1.ConfigMap) string {
	if configMap == nil || configMap.Data == nil {
		return ""
	}

	hash := sha256.New()
	// Hash the config.toml content
	if configToml, ok := configMap.Data["config.toml"]; ok {
		hash.Write([]byte(configToml))
	}

	return hex.EncodeToString(hash.Sum(nil))
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

// isDeploymentReady checks if a Deployment is ready
func isDeploymentReady(deployment *appsv1.Deployment) bool {
	if deployment == nil {
		return false
	}

	// Check if the deployment has the desired number of replicas
	if deployment.Spec.Replicas == nil {
		return false
	}

	desiredReplicas := *deployment.Spec.Replicas

	// All replicas must be ready and updated
	return deployment.Status.ReadyReplicas == desiredReplicas &&
		deployment.Status.UpdatedReplicas == desiredReplicas &&
		deployment.Status.AvailableReplicas == desiredReplicas
}

// isStatefulSetReady checks if a StatefulSet is ready
func isStatefulSetReady(sts *appsv1.StatefulSet) bool {
	if sts == nil {
		return false
	}

	if sts.Spec.Replicas == nil {
		return false
	}

	desiredReplicas := *sts.Spec.Replicas

	return sts.Status.ReadyReplicas == desiredReplicas &&
		sts.Status.CurrentReplicas == desiredReplicas &&
		sts.Status.UpdatedReplicas == desiredReplicas
}

// isServiceReady checks if a Service is ready
func isServiceReady(service *corev1.Service) bool {
	if service == nil {
		return false
	}

	// For ClusterIP and NodePort services, they're ready once created
	if service.Spec.Type == corev1.ServiceTypeClusterIP ||
		service.Spec.Type == corev1.ServiceTypeNodePort {
		return service.Spec.ClusterIP != ""
	}

	// For LoadBalancer services, check if external IP is assigned
	if service.Spec.Type == corev1.ServiceTypeLoadBalancer {
		return len(service.Status.LoadBalancer.Ingress) > 0
	}

	return true
}

// isIngressReady checks if an Ingress is ready
func isIngressReady(ingress *networkingv1.Ingress) bool {
	if ingress == nil {
		return false
	}

	// Check if load balancer IP/hostname is assigned
	return len(ingress.Status.LoadBalancer.Ingress) > 0
}

// isPVCBound checks if a PVC is bound
func isPVCBound(pvc *corev1.PersistentVolumeClaim) bool {
	if pvc == nil {
		return false
	}
	return pvc.Status.Phase == corev1.ClaimBound
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
