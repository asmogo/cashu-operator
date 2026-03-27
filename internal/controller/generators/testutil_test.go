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

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	mintv1alpha1 "github.com/asmogo/cashu-operator/api/v1alpha1"
)

// testScheme returns a scheme with the CashuMint types registered, suitable for
// unit tests that call controllerutil.SetControllerReference.
func testScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(s))
	utilruntime.Must(mintv1alpha1.AddToScheme(s))
	return s
}

// int32Ptr returns a pointer to an int32.
func int32Ptr(i int32) *int32 {
	return &i
}

// float64Ptr returns a pointer to a float64.
func float64Ptr(f float64) *float64 {
	return &f
}

// stringPtr returns a pointer to a string.
func stringPtr(s string) *string {
	return &s
}
