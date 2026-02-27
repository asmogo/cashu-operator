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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

// applyResource creates or updates a resource using server-side apply
func applyResource(ctx context.Context, c client.Client, obj client.Object) error {
	return c.Patch(ctx, obj, client.Apply, client.ForceOwnership, client.FieldOwner("cashu-operator"))
}
