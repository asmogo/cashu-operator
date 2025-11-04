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

package lightning

import (
	"testing"

	mintv1alpha1 "github.com/asmogo/cashu-operator/api/v1alpha1"
)

// TestLNDCanHandle tests LND reconciler's CanHandle method
func TestLNDCanHandle(t *testing.T) {
	tests := []struct {
		name     string
		lnConfig *mintv1alpha1.LightningConfig
		expected bool
	}{
		{
			name:     "nil_lightning_config",
			lnConfig: nil,
			expected: false,
		},
		{
			name: "lnd_backend",
			lnConfig: &mintv1alpha1.LightningConfig{
				Backend: "lnd",
			},
			expected: true,
		},
		{
			name: "cln_backend",
			lnConfig: &mintv1alpha1.LightningConfig{
				Backend: "cln",
			},
			expected: false,
		},
		{
			name: "lnbits_backend",
			lnConfig: &mintv1alpha1.LightningConfig{
				Backend: "lnbits",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reconciler := &LNDReconciler{
				BaseReconciler: &BaseReconciler{},
			}
			result := reconciler.CanHandle(tt.lnConfig)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestLNDReconcilerName tests the Name method
func TestLNDReconcilerName(t *testing.T) {
	reconciler := &LNDReconciler{
		BaseReconciler: &BaseReconciler{},
	}
	expected := "LND"
	if reconciler.Name() != expected {
		t.Errorf("expected %q, got %q", expected, reconciler.Name())
	}
}

// TestLNDCanHandleWithValidConfig tests that LND reconciler validates config properly
func TestLNDCanHandleWithValidConfig(t *testing.T) {
	lnConfig := &mintv1alpha1.LightningConfig{
		Backend: "lnd",
	}

	reconciler := &LNDReconciler{
		BaseReconciler: &BaseReconciler{},
	}

	// Should handle LND config
	if !reconciler.CanHandle(lnConfig) {
		t.Error("expected CanHandle to return true for lnd backend")
	}
}

// TestCLNCanHandle tests Core Lightning reconciler's CanHandle method
func TestCLNCanHandle(t *testing.T) {
	tests := []struct {
		name     string
		lnConfig *mintv1alpha1.LightningConfig
		expected bool
	}{
		{
			name:     "nil_lightning_config",
			lnConfig: nil,
			expected: false,
		},
		{
			name: "lnd_backend",
			lnConfig: &mintv1alpha1.LightningConfig{
				Backend: "lnd",
			},
			expected: false,
		},
		{
			name: "cln_backend",
			lnConfig: &mintv1alpha1.LightningConfig{
				Backend: "cln",
			},
			expected: true,
		},
		{
			name: "lnbits_backend",
			lnConfig: &mintv1alpha1.LightningConfig{
				Backend: "lnbits",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reconciler := &CLNReconciler{
				BaseReconciler: &BaseReconciler{},
			}
			result := reconciler.CanHandle(tt.lnConfig)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestCLNReconcilerName tests the Name method
func TestCLNReconcilerName(t *testing.T) {
	reconciler := &CLNReconciler{
		BaseReconciler: &BaseReconciler{},
	}
	expected := "CoreLightning"
	if reconciler.Name() != expected {
		t.Errorf("expected %q, got %q", expected, reconciler.Name())
	}
}

// TestCLNCanHandleWithValidConfig tests that CLN reconciler validates config properly
func TestCLNCanHandleWithValidConfig(t *testing.T) {
	lnConfig := &mintv1alpha1.LightningConfig{
		Backend: "cln",
	}

	reconciler := &CLNReconciler{
		BaseReconciler: &BaseReconciler{},
	}

	// Should handle CLN config
	if !reconciler.CanHandle(lnConfig) {
		t.Error("expected CanHandle to return true for cln backend")
	}
}

// TestLNbitsCanHandle tests LNbits reconciler's CanHandle method
func TestLNbitsCanHandle(t *testing.T) {
	tests := []struct {
		name     string
		lnConfig *mintv1alpha1.LightningConfig
		expected bool
	}{
		{
			name:     "nil_lightning_config",
			lnConfig: nil,
			expected: false,
		},
		{
			name: "lnd_backend",
			lnConfig: &mintv1alpha1.LightningConfig{
				Backend: "lnd",
			},
			expected: false,
		},
		{
			name: "cln_backend",
			lnConfig: &mintv1alpha1.LightningConfig{
				Backend: "cln",
			},
			expected: false,
		},
		{
			name: "lnbits_backend",
			lnConfig: &mintv1alpha1.LightningConfig{
				Backend: "lnbits",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reconciler := &LNbitsReconciler{
				BaseReconciler: &BaseReconciler{},
			}
			result := reconciler.CanHandle(tt.lnConfig)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestLNbitsReconcilerName tests the Name method
func TestLNbitsReconcilerName(t *testing.T) {
	reconciler := &LNbitsReconciler{
		BaseReconciler: &BaseReconciler{},
	}
	expected := "LNbits"
	if reconciler.Name() != expected {
		t.Errorf("expected %q, got %q", expected, reconciler.Name())
	}
}

// TestLNbitsCanHandleWithValidConfig tests that LNbits reconciler validates config properly
func TestLNbitsCanHandleWithValidConfig(t *testing.T) {
	lnConfig := &mintv1alpha1.LightningConfig{
		Backend: "lnbits",
	}

	reconciler := &LNbitsReconciler{
		BaseReconciler: &BaseReconciler{},
	}

	// Should handle LNbits config
	if !reconciler.CanHandle(lnConfig) {
		t.Error("expected CanHandle to return true for lnbits backend")
	}
}

// TestBaseReconcilerRequeue tests the requeue helper methods
func TestLightningBaseReconcilerRequeue(t *testing.T) {
	reconciler := &BaseReconciler{}

	// Test short requeue
	result, err := reconciler.RequeueAfterShort()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result.RequeueAfter == 0 {
		t.Error("expected non-zero RequeueAfter")
	}

	// Test long requeue
	result, err = reconciler.RequeueAfterLong()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result.RequeueAfter == 0 {
		t.Error("expected non-zero RequeueAfter")
	}
}
