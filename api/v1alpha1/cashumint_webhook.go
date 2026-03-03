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
	r.defaultMintInfo()
	r.defaultDatabase()
	r.defaultIngress()
	r.defaultOperational()
	r.defaultLightning()
}

func (r *CashuMint) defaultMintInfo() {
	if r.Spec.MintInfo.ListenHost == "" {
		r.Spec.MintInfo.ListenHost = "0.0.0.0"
	}
	if r.Spec.MintInfo.ListenPort == 0 {
		r.Spec.MintInfo.ListenPort = 8085
	}
	if r.Spec.Image == "" {
		r.Spec.Image = "ghcr.io/cashubtc/cdk-mintd:latest"
	}
	if r.Spec.Replicas == nil {
		replicas := int32(1)
		r.Spec.Replicas = &replicas
	}
}

func (r *CashuMint) defaultDatabase() {
	if r.Spec.Database.Engine == "" {
		r.Spec.Database.Engine = DatabaseEnginePostgres
	}
	if r.Spec.Database.Engine == DatabaseEnginePostgres && r.Spec.Database.Postgres != nil {
		r.defaultPostgres()
	}
	if r.Spec.Database.Engine == DatabaseEngineSQLite && r.Spec.Database.SQLite != nil {
		if r.Spec.Database.SQLite.DataDir == "" {
			r.Spec.Database.SQLite.DataDir = "/data"
		}
	}
}

func (r *CashuMint) defaultPostgres() {
	pg := r.Spec.Database.Postgres
	if pg.TLSMode == "" {
		if pg.AutoProvision {
			pg.TLSMode = "disable"
		} else {
			pg.TLSMode = "require"
		}
	}
	if pg.MaxConnections == nil {
		maxConn := int32(20)
		pg.MaxConnections = &maxConn
	}
	if pg.ConnectionTimeoutSeconds == nil {
		timeout := int32(10)
		pg.ConnectionTimeoutSeconds = &timeout
	}
	if pg.AutoProvision && pg.AutoProvisionSpec != nil {
		if pg.AutoProvisionSpec.StorageSize == "" {
			pg.AutoProvisionSpec.StorageSize = "10Gi"
		}
		if pg.AutoProvisionSpec.Version == "" {
			pg.AutoProvisionSpec.Version = "15"
		}
	}
}

func (r *CashuMint) defaultIngress() {
	if r.Spec.Ingress == nil || !r.Spec.Ingress.Enabled {
		return
	}
	if r.Spec.Ingress.ClassName == "" {
		r.Spec.Ingress.ClassName = "nginx"
	}
	if r.Spec.Ingress.TLS != nil && r.Spec.Ingress.TLS.CertManager != nil {
		if r.Spec.Ingress.TLS.CertManager.IssuerKind == "" {
			r.Spec.Ingress.TLS.CertManager.IssuerKind = "ClusterIssuer"
		}
	}
}

func (r *CashuMint) defaultOperational() {
	if r.Spec.Logging != nil {
		if r.Spec.Logging.Level == "" {
			r.Spec.Logging.Level = "info"
		}
		if r.Spec.Logging.Format == "" {
			r.Spec.Logging.Format = "json"
		}
	}
	if r.Spec.Storage != nil && r.Spec.Storage.Size == "" {
		r.Spec.Storage.Size = "10Gi"
	}
	if r.Spec.Service != nil && r.Spec.Service.Type == "" {
		r.Spec.Service.Type = "ClusterIP"
	}
	if r.Spec.HTTPCache != nil {
		r.defaultHTTPCache()
	}
	if r.Spec.ManagementRPC != nil && r.Spec.ManagementRPC.Enabled {
		if r.Spec.ManagementRPC.Address == "" {
			r.Spec.ManagementRPC.Address = "127.0.0.1"
		}
		if r.Spec.ManagementRPC.Port == 0 {
			r.Spec.ManagementRPC.Port = 8086
		}
	}
	if r.Spec.Backup != nil && r.Spec.Backup.Enabled {
		r.defaultBackup()
	}
	if r.Spec.Auth != nil && r.Spec.Auth.Enabled {
		r.defaultAuth()
	}
}

func (r *CashuMint) defaultHTTPCache() {
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

func (r *CashuMint) defaultBackup() {
	if r.Spec.Backup.Schedule == "" {
		r.Spec.Backup.Schedule = "0 */6 * * *"
	}
	if r.Spec.Backup.RetentionCount == nil {
		retention := int32(14)
		r.Spec.Backup.RetentionCount = &retention
	}
	if r.Spec.Backup.S3 != nil && r.Spec.Backup.S3.Prefix == "" {
		r.Spec.Backup.S3.Prefix = r.Name
	}
}

func (r *CashuMint) defaultAuth() {
	boolTrue := true
	if r.Spec.Auth.MintMaxBat == nil {
		maxBat := int32(50)
		r.Spec.Auth.MintMaxBat = &maxBat
	}
	if r.Spec.Auth.EnabledMint == nil {
		r.Spec.Auth.EnabledMint = &boolTrue
	}
	if r.Spec.Auth.EnabledMelt == nil {
		r.Spec.Auth.EnabledMelt = &boolTrue
	}
	if r.Spec.Auth.EnabledSwap == nil {
		r.Spec.Auth.EnabledSwap = &boolTrue
	}
	if r.Spec.Auth.EnabledCheckMintQuote == nil {
		r.Spec.Auth.EnabledCheckMintQuote = &boolTrue
	}
	if r.Spec.Auth.EnabledCheckMeltQuote == nil {
		r.Spec.Auth.EnabledCheckMeltQuote = &boolTrue
	}
	if r.Spec.Auth.EnabledRestore == nil {
		r.Spec.Auth.EnabledRestore = &boolTrue
	}
}

func (r *CashuMint) defaultLightning() {
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
		r.defaultFakeWallet()
	}
	if r.Spec.Lightning.GRPCProcessor != nil {
		r.defaultGRPCProcessor()
	}
	if r.Spec.LDKNode != nil && r.Spec.LDKNode.Enabled {
		r.defaultLDKNode()
	}
}

func (r *CashuMint) defaultFakeWallet() {
	fw := r.Spec.Lightning.FakeWallet
	if fw.FeePercent == nil {
		fee := 0.02
		fw.FeePercent = &fee
	}
	if fw.ReserveFeeMin == nil {
		reserveFee := int32(1)
		fw.ReserveFeeMin = &reserveFee
	}
	if fw.MinDelayTime == nil {
		minDelay := int32(1)
		fw.MinDelayTime = &minDelay
	}
	if fw.MaxDelayTime == nil {
		maxDelay := int32(3)
		fw.MaxDelayTime = &maxDelay
	}
	if len(fw.SupportedUnits) == 0 {
		fw.SupportedUnits = []string{"sat"}
	}
}

func (r *CashuMint) defaultGRPCProcessor() {
	gp := r.Spec.Lightning.GRPCProcessor
	if len(gp.SupportedUnits) == 0 {
		gp.SupportedUnits = []string{"sat"}
	}
	if gp.Port == 0 {
		gp.Port = 50051
	}
	if gp.SidecarProcessor != nil && gp.SidecarProcessor.Enabled {
		if gp.SidecarProcessor.ImagePullPolicy == "" {
			gp.SidecarProcessor.ImagePullPolicy = "IfNotPresent"
		}
	}
}

func (r *CashuMint) defaultLDKNode() {
	ldk := r.Spec.LDKNode
	if ldk.FeePercent == nil {
		fee := 0.04
		ldk.FeePercent = &fee
	}
	if ldk.ReserveFeeMin == nil {
		reserveFee := int32(4)
		ldk.ReserveFeeMin = &reserveFee
	}
	if ldk.BitcoinNetwork == "" {
		ldk.BitcoinNetwork = "signet"
	}
	if ldk.ChainSourceType == "" {
		ldk.ChainSourceType = "esplora"
	}
	if ldk.Host == "" {
		ldk.Host = "0.0.0.0"
	}
	if ldk.Port == 0 {
		ldk.Port = 8090
	}
	if ldk.GossipSourceType == "" {
		ldk.GossipSourceType = "rgs"
	}
	if ldk.WebserverHost == "" {
		ldk.WebserverHost = "127.0.0.1"
	}
	if ldk.WebserverPort == 0 {
		ldk.WebserverPort = 8888
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

	// Validate Backup configuration
	if err := r.validateBackup(); err != nil {
		allErrs = append(allErrs, err)
	}

	if len(allErrs) > 0 {
		return fmt.Errorf("validation failed: %v", allErrs)
	}

	return nil
}

// validateBackup validates the backup configuration
func (r *CashuMint) validateBackup() error {
	if r.Spec.Backup == nil || !r.Spec.Backup.Enabled {
		return nil
	}

	var errs []error

	if r.Spec.Database.Engine != "postgres" {
		errs = append(errs, fmt.Errorf("spec.backup.enabled requires spec.database.engine to be postgres"))
	}
	if r.Spec.Database.Postgres == nil || !r.Spec.Database.Postgres.AutoProvision {
		errs = append(errs, fmt.Errorf("spec.backup.enabled currently requires spec.database.postgres.autoProvision=true"))
	}

	if r.Spec.Backup.Schedule == "" {
		errs = append(errs, fmt.Errorf("spec.backup.schedule is required when backup is enabled"))
	}

	if r.Spec.Backup.S3 == nil {
		errs = append(errs, fmt.Errorf("spec.backup.s3 is required when backup is enabled"))
	} else {
		if r.Spec.Backup.S3.Bucket == "" {
			errs = append(errs, fmt.Errorf("spec.backup.s3.bucket is required"))
		}
		if r.Spec.Backup.S3.AccessKeyIDSecretRef.Name == "" ||
			r.Spec.Backup.S3.AccessKeyIDSecretRef.Key == "" {
			errs = append(errs, fmt.Errorf("spec.backup.s3.accessKeyIdSecretRef.name and key are required"))
		}
		if r.Spec.Backup.S3.SecretAccessKeySecretRef.Name == "" ||
			r.Spec.Backup.S3.SecretAccessKeySecretRef.Key == "" {
			errs = append(errs, fmt.Errorf("spec.backup.s3.secretAccessKeySecretRef.name and key are required"))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("backup validation errors: %v", errs)
	}

	return nil
}

// validateDatabase validates the database configuration
func (r *CashuMint) validateDatabase() error {
	var errs []error

	switch r.Spec.Database.Engine {
	case DatabaseEnginePostgres:
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
	case DatabaseEngineSQLite, DatabaseEngineRedb:
		// No additional validation needed for local engines.
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
	case LightningBackendLND:
		if r.Spec.Lightning.LND == nil {
			errs = append(errs, fmt.Errorf("spec.lightning.lnd is required when backend is lnd"))
		} else {
			if r.Spec.Lightning.LND.Address == "" {
				errs = append(errs, fmt.Errorf("spec.lightning.lnd.address is required"))
			}
		}
	case LightningBackendCLN:
		if r.Spec.Lightning.CLN == nil {
			errs = append(errs, fmt.Errorf("spec.lightning.cln is required when backend is cln"))
		} else {
			if r.Spec.Lightning.CLN.RPCPath == "" {
				errs = append(errs, fmt.Errorf("spec.lightning.cln.rpcPath is required"))
			}
		}
	case LightningBackendLNBits:
		if r.Spec.Lightning.LNBits == nil {
			errs = append(errs, fmt.Errorf("spec.lightning.lnbits is required when backend is lnbits"))
		} else {
			if r.Spec.Lightning.LNBits.API == "" {
				errs = append(errs, fmt.Errorf("spec.lightning.lnbits.api is required"))
			}
		}
	case LightningBackendFakeWallet:
		if r.Spec.Lightning.FakeWallet == nil {
			errs = append(errs, fmt.Errorf("spec.lightning.fakeWallet is required when backend is fakewallet"))
		}
	case LightningBackendGRPCProcessor:
		if r.Spec.Lightning.GRPCProcessor == nil {
			errs = append(errs, fmt.Errorf("spec.lightning.grpcProcessor is required when backend is grpcprocessor"))
		} else {
			// If a sidecar processor is enabled, address can be omitted (defaults to localhost)
			sidecarEnabled := r.Spec.Lightning.GRPCProcessor.SidecarProcessor != nil &&
				r.Spec.Lightning.GRPCProcessor.SidecarProcessor.Enabled

			if !sidecarEnabled && r.Spec.Lightning.GRPCProcessor.Address == "" {
				errs = append(errs, fmt.Errorf("spec.lightning.grpcProcessor.address is required when sidecarProcessor is not enabled"))
			}

			// Validate sidecar processor configuration if enabled
			if sidecarEnabled {
				sidecar := r.Spec.Lightning.GRPCProcessor.SidecarProcessor
				if sidecar.Image == "" {
					errs = append(errs, fmt.Errorf("spec.lightning.grpcProcessor.sidecarProcessor.image is required when enabled"))
				}
				if sidecar.EnableTLS && sidecar.TLSSecretRef == nil {
					errs = append(errs, fmt.Errorf("spec.lightning.grpcProcessor.sidecarProcessor.tlsSecretRef is required when enableTLS is true"))
				}
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
