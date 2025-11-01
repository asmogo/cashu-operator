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
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var cashumintlog = logf.Log.WithName("cashumint-resource")

// SetupWebhookWithManager sets up the webhook with the Manager.
func (r *CashuMint) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-mint-cashu-asmogo-github-io-v1alpha1-cashumint,mutating=true,failurePolicy=fail,sideEffects=None,groups=mint.cashu.asmogo.github.io,resources=cashumints,verbs=create;update,versions=v1alpha1,name=mcashumint.kb.io,admissionReviewVersions=v1

// Default implements defaulting for CashuMint
func (r *CashuMint) Default() {
	cashumintlog.Info("default", "name", r.Name)

	// Apply defaults for MintInfo
	if r.Spec.MintInfo.ListenHost == "" {
		r.Spec.MintInfo.ListenHost = "0.0.0.0"
	}
	if r.Spec.MintInfo.ListenPort == 0 {
		r.Spec.MintInfo.ListenPort = 8085
	}

	// Apply defaults for Image
	if r.Spec.Image == "" {
		r.Spec.Image = "cashubtc/mintd:latest"
	}

	// Apply defaults for Replicas
	if r.Spec.Replicas == nil {
		replicas := int32(1)
		r.Spec.Replicas = &replicas
	}

	// Apply defaults for Database
	if r.Spec.Database.Engine == "" {
		r.Spec.Database.Engine = "postgres"
	}

	// Apply defaults for PostgreSQL if engine is postgres
	if r.Spec.Database.Engine == "postgres" && r.Spec.Database.Postgres != nil {
		if r.Spec.Database.Postgres.TLSMode == "" {
			r.Spec.Database.Postgres.TLSMode = "require"
		}
		if r.Spec.Database.Postgres.MaxConnections == nil {
			maxConn := int32(20)
			r.Spec.Database.Postgres.MaxConnections = &maxConn
		}
		if r.Spec.Database.Postgres.ConnectionTimeoutSeconds == nil {
			timeout := int32(10)
			r.Spec.Database.Postgres.ConnectionTimeoutSeconds = &timeout
		}

		// Apply defaults for auto-provisioning if enabled
		if r.Spec.Database.Postgres.AutoProvision && r.Spec.Database.Postgres.AutoProvisionSpec != nil {
			if r.Spec.Database.Postgres.AutoProvisionSpec.StorageSize == "" {
				r.Spec.Database.Postgres.AutoProvisionSpec.StorageSize = "10Gi"
			}
			if r.Spec.Database.Postgres.AutoProvisionSpec.Version == "" {
				r.Spec.Database.Postgres.AutoProvisionSpec.Version = "15"
			}
		}
	}

	// Apply defaults for SQLite if engine is sqlite
	if r.Spec.Database.Engine == "sqlite" && r.Spec.Database.SQLite != nil {
		if r.Spec.Database.SQLite.DataDir == "" {
			r.Spec.Database.SQLite.DataDir = "/data"
		}
	}

	// Apply defaults for Ingress
	if r.Spec.Ingress != nil && r.Spec.Ingress.Enabled {
		if r.Spec.Ingress.ClassName == "" {
			r.Spec.Ingress.ClassName = "nginx"
		}
		if r.Spec.Ingress.TLS != nil && r.Spec.Ingress.TLS.CertManager != nil {
			if r.Spec.Ingress.TLS.CertManager.IssuerKind == "" {
				r.Spec.Ingress.TLS.CertManager.IssuerKind = "ClusterIssuer"
			}
		}
	}

	// Apply defaults for Logging
	if r.Spec.Logging != nil {
		if r.Spec.Logging.Level == "" {
			r.Spec.Logging.Level = "info"
		}
		if r.Spec.Logging.Format == "" {
			r.Spec.Logging.Format = "json"
		}
	}

	// Apply defaults for Storage
	if r.Spec.Storage != nil && r.Spec.Storage.Size == "" {
		r.Spec.Storage.Size = "10Gi"
	}

	// Apply defaults for Service
	if r.Spec.Service != nil && r.Spec.Service.Type == "" {
		r.Spec.Service.Type = "ClusterIP"
	}

	// Apply defaults for HTTPCache
	if r.Spec.HTTPCache != nil {
		if r.Spec.HTTPCache.Backend == "" {
			r.Spec.HTTPCache.Backend = "memory"
		}
		if r.Spec.HTTPCache.TTL == nil {
			ttl := int32(60)
			r.Spec.HTTPCache.TTL = &ttl
		}
		if r.Spec.HTTPCache.TTI == nil {
			tti := int32(60)
			r.Spec.HTTPCache.TTI = &tti
		}
	}

	// Apply defaults for ManagementRPC
	if r.Spec.ManagementRPC != nil && r.Spec.ManagementRPC.Enabled {
		if r.Spec.ManagementRPC.Address == "" {
			r.Spec.ManagementRPC.Address = "127.0.0.1"
		}
		if r.Spec.ManagementRPC.Port == 0 {
			r.Spec.ManagementRPC.Port = 8086
		}
	}

	// Apply defaults for Auth
	if r.Spec.Auth != nil && r.Spec.Auth.Enabled {
		if r.Spec.Auth.MintMaxBat == nil {
			maxBat := int32(50)
			r.Spec.Auth.MintMaxBat = &maxBat
		}
		if r.Spec.Auth.EnabledMint == nil {
			enabled := true
			r.Spec.Auth.EnabledMint = &enabled
		}
		if r.Spec.Auth.EnabledMelt == nil {
			enabled := true
			r.Spec.Auth.EnabledMelt = &enabled
		}
		if r.Spec.Auth.EnabledSwap == nil {
			enabled := true
			r.Spec.Auth.EnabledSwap = &enabled
		}
		if r.Spec.Auth.EnabledCheckMintQuote == nil {
			enabled := true
			r.Spec.Auth.EnabledCheckMintQuote = &enabled
		}
		if r.Spec.Auth.EnabledCheckMeltQuote == nil {
			enabled := true
			r.Spec.Auth.EnabledCheckMeltQuote = &enabled
		}
		if r.Spec.Auth.EnabledRestore == nil {
			enabled := true
			r.Spec.Auth.EnabledRestore = &enabled
		}
	}

	// Apply defaults for Lightning backends
	if r.Spec.Lightning.LND != nil {
		if r.Spec.Lightning.LND.FeePercent == nil {
			fee := 0.04
			r.Spec.Lightning.LND.FeePercent = &fee
		}
		if r.Spec.Lightning.LND.ReserveFeeMin == nil {
			reserveFee := int32(4)
			r.Spec.Lightning.LND.ReserveFeeMin = &reserveFee
		}
	}

	if r.Spec.Lightning.CLN != nil {
		if r.Spec.Lightning.CLN.FeePercent == nil {
			fee := 0.04
			r.Spec.Lightning.CLN.FeePercent = &fee
		}
		if r.Spec.Lightning.CLN.ReserveFeeMin == nil {
			reserveFee := int32(4)
			r.Spec.Lightning.CLN.ReserveFeeMin = &reserveFee
		}
	}

	if r.Spec.Lightning.FakeWallet != nil {
		if r.Spec.Lightning.FakeWallet.FeePercent == nil {
			fee := 0.02
			r.Spec.Lightning.FakeWallet.FeePercent = &fee
		}
		if r.Spec.Lightning.FakeWallet.ReserveFeeMin == nil {
			reserveFee := int32(1)
			r.Spec.Lightning.FakeWallet.ReserveFeeMin = &reserveFee
		}
		if r.Spec.Lightning.FakeWallet.MinDelayTime == nil {
			minDelay := int32(1)
			r.Spec.Lightning.FakeWallet.MinDelayTime = &minDelay
		}
		if r.Spec.Lightning.FakeWallet.MaxDelayTime == nil {
			maxDelay := int32(3)
			r.Spec.Lightning.FakeWallet.MaxDelayTime = &maxDelay
		}
		if len(r.Spec.Lightning.FakeWallet.SupportedUnits) == 0 {
			r.Spec.Lightning.FakeWallet.SupportedUnits = []string{"sat"}
		}
	}

	if r.Spec.Lightning.GRPCProcessor != nil {
		if len(r.Spec.Lightning.GRPCProcessor.SupportedUnits) == 0 {
			r.Spec.Lightning.GRPCProcessor.SupportedUnits = []string{"sat"}
		}
	}

	// Apply defaults for LDK Node
	if r.Spec.LDKNode != nil && r.Spec.LDKNode.Enabled {
		if r.Spec.LDKNode.FeePercent == nil {
			fee := 0.04
			r.Spec.LDKNode.FeePercent = &fee
		}
		if r.Spec.LDKNode.ReserveFeeMin == nil {
			reserveFee := int32(4)
			r.Spec.LDKNode.ReserveFeeMin = &reserveFee
		}
		if r.Spec.LDKNode.BitcoinNetwork == "" {
			r.Spec.LDKNode.BitcoinNetwork = "signet"
		}
		if r.Spec.LDKNode.ChainSourceType == "" {
			r.Spec.LDKNode.ChainSourceType = "esplora"
		}
		if r.Spec.LDKNode.Host == "" {
			r.Spec.LDKNode.Host = "0.0.0.0"
		}
		if r.Spec.LDKNode.Port == 0 {
			r.Spec.LDKNode.Port = 8090
		}
		if r.Spec.LDKNode.GossipSourceType == "" {
			r.Spec.LDKNode.GossipSourceType = "rgs"
		}
		if r.Spec.LDKNode.WebserverHost == "" {
			r.Spec.LDKNode.WebserverHost = "127.0.0.1"
		}
		if r.Spec.LDKNode.WebserverPort == 0 {
			r.Spec.LDKNode.WebserverPort = 8888
		}
	}
}

// +kubebuilder:webhook:path=/validate-mint-cashu-asmogo-github-io-v1alpha1-cashumint,mutating=false,failurePolicy=fail,sideEffects=None,groups=mint.cashu.asmogo.github.io,resources=cashumints,verbs=create;update,versions=v1alpha1,name=vcashumint.kb.io,admissionReviewVersions=v1

// ValidateCreate implements validation for CashuMint creation
func (r *CashuMint) ValidateCreate() (admission.Warnings, error) {
	cashumintlog.Info("validate create", "name", r.Name)

	return nil, r.validateCashuMint()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *CashuMint) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	cashumintlog.Info("validate update", "name", r.Name)

	return nil, r.validateCashuMint()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *CashuMint) ValidateDelete() (admission.Warnings, error) {
	cashumintlog.Info("validate delete", "name", r.Name)

	// No validation needed for delete
	return nil, nil
}

// validateCashuMint performs validation on the CashuMint resource
func (r *CashuMint) validateCashuMint() error {
	var allErrs []error

	// Validate MintInfo
	if r.Spec.MintInfo.URL == "" {
		allErrs = append(allErrs, fmt.Errorf("spec.mintInfo.url is required"))
	}

	// Validate Database configuration
	if err := r.validateDatabase(); err != nil {
		allErrs = append(allErrs, err)
	}

	// Validate Lightning configuration
	if err := r.validateLightning(); err != nil {
		allErrs = append(allErrs, err)
	}

	// Validate Ingress configuration
	if err := r.validateIngress(); err != nil {
		allErrs = append(allErrs, err)
	}

	// Validate Resources if specified
	if err := r.validateResources(); err != nil {
		allErrs = append(allErrs, err)
	}

	if len(allErrs) > 0 {
		return fmt.Errorf("validation failed: %v", allErrs)
	}

	return nil
}

// validateDatabase validates the database configuration
func (r *CashuMint) validateDatabase() error {
	var errs []error

	switch r.Spec.Database.Engine {
	case "postgres":
		if r.Spec.Database.Postgres == nil {
			errs = append(errs, fmt.Errorf("spec.database.postgres is required when engine is postgres"))
		} else {
			// Must have either URL or URLSecretRef, or AutoProvision enabled
			hasURL := r.Spec.Database.Postgres.URL != ""
			hasSecretRef := r.Spec.Database.Postgres.URLSecretRef != nil
			autoProvision := r.Spec.Database.Postgres.AutoProvision

			if !autoProvision && !hasURL && !hasSecretRef {
				errs = append(errs, fmt.Errorf("spec.database.postgres must have either url, urlSecretRef, or autoProvision enabled"))
			}

			// Cannot have both URL and URLSecretRef
			if hasURL && hasSecretRef {
				errs = append(errs, fmt.Errorf("spec.database.postgres cannot specify both url and urlSecretRef"))
			}
		}
	case "sqlite", "redb":
		// For SQLite and redb, ensure storage is configured
		if r.Spec.Storage == nil {
			// This is acceptable as we apply defaults
		}
	default:
		errs = append(errs, fmt.Errorf("invalid database engine: %s (must be postgres, sqlite, or redb)", r.Spec.Database.Engine))
	}

	if len(errs) > 0 {
		return fmt.Errorf("database validation errors: %v", errs)
	}
	return nil
}

// validateLightning validates the Lightning backend configuration
func (r *CashuMint) validateLightning() error {
	var errs []error

	switch r.Spec.Lightning.Backend {
	case "lnd":
		if r.Spec.Lightning.LND == nil {
			errs = append(errs, fmt.Errorf("spec.lightning.lnd is required when backend is lnd"))
		} else {
			if r.Spec.Lightning.LND.Address == "" {
				errs = append(errs, fmt.Errorf("spec.lightning.lnd.address is required"))
			}
		}
	case "cln":
		if r.Spec.Lightning.CLN == nil {
			errs = append(errs, fmt.Errorf("spec.lightning.cln is required when backend is cln"))
		} else {
			if r.Spec.Lightning.CLN.RPCPath == "" {
				errs = append(errs, fmt.Errorf("spec.lightning.cln.rpcPath is required"))
			}
		}
	case "lnbits":
		if r.Spec.Lightning.LNBits == nil {
			errs = append(errs, fmt.Errorf("spec.lightning.lnbits is required when backend is lnbits"))
		} else {
			if r.Spec.Lightning.LNBits.API == "" {
				errs = append(errs, fmt.Errorf("spec.lightning.lnbits.api is required"))
			}
		}
	case "fakewallet":
		if r.Spec.Lightning.FakeWallet == nil {
			errs = append(errs, fmt.Errorf("spec.lightning.fakeWallet is required when backend is fakewallet"))
		}
	case "grpcprocessor":
		if r.Spec.Lightning.GRPCProcessor == nil {
			errs = append(errs, fmt.Errorf("spec.lightning.grpcProcessor is required when backend is grpcprocessor"))
		} else {
			if r.Spec.Lightning.GRPCProcessor.Address == "" {
				errs = append(errs, fmt.Errorf("spec.lightning.grpcProcessor.address is required"))
			}
			if r.Spec.Lightning.GRPCProcessor.Port == 0 {
				errs = append(errs, fmt.Errorf("spec.lightning.grpcProcessor.port is required"))
			}
		}
	default:
		errs = append(errs, fmt.Errorf("invalid lightning backend: %s (must be lnd, cln, lnbits, fakewallet, or grpcprocessor)", r.Spec.Lightning.Backend))
	}

	if len(errs) > 0 {
		return fmt.Errorf("lightning validation errors: %v", errs)
	}
	return nil
}

// validateIngress validates the ingress configuration
func (r *CashuMint) validateIngress() error {
	if r.Spec.Ingress == nil || !r.Spec.Ingress.Enabled {
		return nil
	}

	var errs []error

	if r.Spec.Ingress.Host == "" {
		errs = append(errs, fmt.Errorf("spec.ingress.host is required when ingress is enabled"))
	}

	// Validate cert-manager configuration if TLS is enabled
	if r.Spec.Ingress.TLS != nil && r.Spec.Ingress.TLS.Enabled {
		if r.Spec.Ingress.TLS.CertManager != nil && r.Spec.Ingress.TLS.CertManager.Enabled {
			if r.Spec.Ingress.TLS.CertManager.IssuerName == "" {
				errs = append(errs, fmt.Errorf("spec.ingress.tls.certManager.issuerName is required when cert-manager is enabled"))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("ingress validation errors: %v", errs)
	}
	return nil
}

// validateResources validates resource requirements if specified
func (r *CashuMint) validateResources() error {
	if r.Spec.Resources == nil {
		return nil
	}

	// Basic validation: ensure requests don't exceed limits
	if r.Spec.Resources.Limits != nil && r.Spec.Resources.Requests != nil {
		for resource, limit := range r.Spec.Resources.Limits {
			if request, ok := r.Spec.Resources.Requests[resource]; ok {
				if request.Cmp(limit) > 0 {
					return fmt.Errorf("resource request for %s exceeds limit", resource)
				}
			}
		}
	}

	return nil
}
