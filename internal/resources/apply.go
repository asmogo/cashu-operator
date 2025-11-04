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

package resources

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	mintv1alpha1 "github.com/asmogo/cashu-operator/api/v1alpha1"
)

// Requeue timing constants for common reconciliation scenarios
const (
	// DefaultRequeueAfterShort is used for quick retries on transient errors
	DefaultRequeueAfterShort = 5 * time.Second

	// DefaultRequeueAfterMedium is used for medium-term retry intervals
	DefaultRequeueAfterMedium = 30 * time.Second

	// DefaultRequeueAfterLong is used for periodic status checks
	DefaultRequeueAfterLong = 2 * time.Minute
)

// Applier provides utilities for applying and managing Kubernetes resources.
type Applier struct {
	client client.Client
	scheme *runtime.Scheme
}

// NewApplier creates a new resource applier.
func NewApplier(c client.Client, scheme *runtime.Scheme) *Applier {
	return &Applier{
		client: c,
		scheme: scheme,
	}
}

// Apply creates or updates a resource using server-side apply.
// This ensures proper ownership and field management.
func (a *Applier) Apply(ctx context.Context, obj client.Object) error {
	return a.client.Patch(ctx, obj, client.Apply, client.ForceOwnership, client.FieldOwner("cashu-operator"))
}

// SetOwnerReference ensures the given object has a controller reference to the mint.
func (a *Applier) SetOwnerReference(owner client.Object, dependent client.Object) error {
	return controllerutil.SetControllerReference(owner, dependent, a.scheme)
}

// Delete removes a resource if it exists, ignoring not-found errors.
func (a *Applier) Delete(ctx context.Context, obj client.Object) error {
	err := a.client.Delete(ctx, obj)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

// Get fetches a resource, returning nil if not found.
func (a *Applier) Get(ctx context.Context, key client.ObjectKey, obj client.Object) error {
	err := a.client.Get(ctx, key, obj)
	if err != nil && apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

// GetOrError fetches a resource, returning an error if not found.
func (a *Applier) GetOrError(ctx context.Context, key client.ObjectKey, obj client.Object) error {
	return a.client.Get(ctx, key, obj)
}

// ApplyWithOwner applies a resource and sets the owner reference in one operation.
func (a *Applier) ApplyWithOwner(ctx context.Context, owner *mintv1alpha1.CashuMint, obj client.Object) error {
	if err := a.SetOwnerReference(owner, obj); err != nil {
		return fmt.Errorf("failed to set owner reference: %w", err)
	}
	return a.Apply(ctx, obj)
}

// ConfigMapHash calculates a SHA256 hash of ConfigMap data.
// This is useful for triggering pod restarts when configuration changes.
func ConfigMapHash(configMap *corev1.ConfigMap) string {
	if configMap == nil || configMap.Data == nil {
		return ""
	}

	hash := sha256.New()
	if configToml, ok := configMap.Data["config.toml"]; ok {
		hash.Write([]byte(configToml))
	}

	return hex.EncodeToString(hash.Sum(nil))
}

// SecretHash calculates a SHA256 hash of Secret data.
// This is useful for triggering pod restarts when secrets change.
func SecretHash(secret *corev1.Secret) string {
	if secret == nil || secret.Data == nil {
		return ""
	}

	hash := sha256.New()
	for _, v := range secret.Data {
		hash.Write(v)
	}

	return hex.EncodeToString(hash.Sum(nil))
}

// MustPatch performs a patch operation, panicking on error.
// Only use in tests or initialization code where failure should be fatal.
func (a *Applier) MustPatch(ctx context.Context, obj client.Object, patch client.Patch) {
	if err := a.client.Patch(ctx, obj, patch); err != nil {
		log.FromContext(ctx).Error(err, "failed to patch resource", "resource", obj.GetName())
		panic(err)
	}
}

// List fetches a list of resources matching the given list options.
func (a *Applier) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return a.client.List(ctx, list, opts...)
}

// CreateNamespacedName is a convenience function for creating a NamespacedName.
func CreateNamespacedName(namespace, name string) types.NamespacedName {
	return types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}
}

// CreateObjectKey is an alias for CreateNamespacedName for clarity in different contexts.
func CreateObjectKey(namespace, name string) client.ObjectKey {
	return client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}
}

// TypeMeta creates a TypeMeta for a Kubernetes resource.
func TypeMeta(apiVersion, kind string) metav1.TypeMeta {
	return metav1.TypeMeta{
		APIVersion: apiVersion,
		Kind:       kind,
	}
}

// ObjectMeta creates an ObjectMeta with namespace and name.
func ObjectMeta(namespace, name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Namespace: namespace,
		Name:      name,
	}
}

// LabelSelector creates a LabelSelector for label matching.
func LabelSelector(labels map[string]string) metav1.LabelSelector {
	return metav1.LabelSelector{
		MatchLabels: labels,
	}
}

// MustParseQuantity parses a resource quantity string, panicking on error.
func MustParseQuantity(q string) resource.Quantity {
	quantity, err := resource.ParseQuantity(q)
	if err != nil {
		panic(fmt.Sprintf("failed to parse quantity %q: %v", q, err))
	}
	return quantity
}

// IntToIntstr converts an int32 to IntOrString for port specifications.
func IntToIntstr(i int32) intstr.IntOrString {
	return intstr.FromInt(int(i))
}
