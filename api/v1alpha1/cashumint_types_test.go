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

package v1alpha1

import (
	"testing"
)

func TestActiveBackend_Single(t *testing.T) {
	tests := []struct {
		name     string
		config   PaymentBackendConfig
		expected string
	}{
		{
			name:     "LND",
			config:   PaymentBackendConfig{LND: &LNDConfig{Address: "https://lnd:10009"}},
			expected: PaymentBackendLND,
		},
		{
			name:     "CLN",
			config:   PaymentBackendConfig{CLN: &CLNConfig{RPCPath: "/rpc"}},
			expected: PaymentBackendCLN,
		},
		{
			name:     "LNBits",
			config:   PaymentBackendConfig{LNBits: &LNBitsConfig{API: "https://lnbits"}},
			expected: PaymentBackendLNBits,
		},
		{
			name:     "FakeWallet",
			config:   PaymentBackendConfig{FakeWallet: &FakeWalletConfig{}},
			expected: PaymentBackendFakeWallet,
		},
		{
			name:     "GRPCProcessor",
			config:   PaymentBackendConfig{GRPCProcessor: &GRPCProcessorConfig{Address: "localhost"}},
			expected: PaymentBackendGRPCProcessor,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.ActiveBackend()
			if result != tt.expected {
				t.Errorf("ActiveBackend() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestActiveBackend_None(t *testing.T) {
	config := PaymentBackendConfig{}
	result := config.ActiveBackend()
	if result != "" {
		t.Errorf("ActiveBackend() = %q, want empty string", result)
	}
}

func TestActiveBackend_Multiple(t *testing.T) {
	config := PaymentBackendConfig{
		LND:        &LNDConfig{Address: "https://lnd:10009"},
		FakeWallet: &FakeWalletConfig{},
	}
	result := config.ActiveBackend()
	if result != "" {
		t.Errorf("ActiveBackend() = %q, want empty string for multiple backends", result)
	}
}
