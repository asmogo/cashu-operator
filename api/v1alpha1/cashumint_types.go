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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CashuMint is the Schema for the cashumints API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=mint;mints
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Backend",type=string,JSONPath=`.spec.lightning.backend`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type CashuMint struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CashuMintSpec   `json:"spec,omitempty"`
	Status CashuMintStatus `json:"status,omitempty"`
}

// CashuMintSpec defines the desired state of CashuMint
type CashuMintSpec struct {
	// Image specifies the container image to use
	// +kubebuilder:default="ghcr.io/cashubtc/cdk-mintd:latest"
	// +optional
	Image string `json:"image,omitempty"`

	// ImagePullPolicy specifies when to pull the image
	// +kubebuilder:default="IfNotPresent"
	// +kubebuilder:validation:Enum=Always;Never;IfNotPresent
	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// ImagePullSecrets is an optional list of references to secrets for pulling images
	// +optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// Replicas is the number of mint instances to run
	// For production, this should be 1 as the mint is stateful
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=1
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// MintInfo contains metadata about the mint
	MintInfo MintInfo `json:"mintInfo"`

	// Database specifies the database backend configuration
	Database DatabaseConfig `json:"database"`

	// Lightning specifies the Lightning Network backend configuration
	Lightning LightningConfig `json:"lightning"`

	// LDKNode specifies optional LDK node configuration
	// +optional
	LDKNode *LDKNodeConfig `json:"ldkNode,omitempty"`

	// Auth specifies authentication configuration
	// +optional
	Auth *AuthConfig `json:"auth,omitempty"`

	// HTTPCache specifies HTTP cache configuration
	// +optional
	HTTPCache *HTTPCacheConfig `json:"httpCache,omitempty"`

	// ManagementRPC enables the management RPC interface
	// +optional
	ManagementRPC *ManagementRPCConfig `json:"managementRPC,omitempty"`

	// Ingress specifies ingress configuration
	// +optional
	Ingress *IngressConfig `json:"ingress,omitempty"`

	// Service specifies service configuration
	// +optional
	Service *ServiceConfig `json:"service,omitempty"`

	// Resources specifies compute resource requirements
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// NodeSelector for pod assignment
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Tolerations for pod assignment
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// Affinity for pod assignment
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// Logging specifies logging configuration
	// +optional
	Logging *LoggingConfig `json:"logging,omitempty"`

	// Storage specifies persistent storage configuration
	// +optional
	Storage *StorageConfig `json:"storage,omitempty"`

	// PodSecurityContext specifies the security context for the pod
	// If not specified, defaults to RunAsNonRoot=true, RunAsUser=1000, FSGroup=1000
	// +optional
	PodSecurityContext *corev1.PodSecurityContext `json:"podSecurityContext,omitempty"`

	// ContainerSecurityContext specifies the security context for the mint container
	// If not specified, defaults to AllowPrivilegeEscalation=false, ReadOnlyRootFilesystem=false, RunAsNonRoot=true
	// +optional
	ContainerSecurityContext *corev1.SecurityContext `json:"containerSecurityContext,omitempty"`
}

// MintInfo contains metadata about the mint
type MintInfo struct {
	// URL is the public URL where the mint is accessible
	// +kubebuilder:validation:Pattern=`^https?://.*`
	URL string `json:"url"`

	// ListenHost is the host to bind to (usually 0.0.0.0 in containers)
	// +kubebuilder:default="0.0.0.0"
	// +optional
	ListenHost string `json:"listenHost,omitempty"`

	// ListenPort is the port the mint API listens on
	// +kubebuilder:default=8085
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +optional
	ListenPort int32 `json:"listenPort,omitempty"`

	// MnemonicSecretRef references a Secret containing the mnemonic
	// Required for production, should never be in plain text
	// +optional
	MnemonicSecretRef *corev1.SecretKeySelector `json:"mnemonicSecretRef,omitempty"`

	// Name is the display name of the mint
	// +optional
	Name string `json:"name,omitempty"`

	// Description is a short description of the mint
	// +optional
	Description string `json:"description,omitempty"`

	// DescriptionLong is a longer description
	// +optional
	DescriptionLong string `json:"descriptionLong,omitempty"`

	// MOTD is the message of the day
	// +optional
	MOTD string `json:"motd,omitempty"`

	// PubkeyHex is the hex pubkey of the mint
	// +optional
	PubkeyHex string `json:"pubkeyHex,omitempty"`

	// IconURL is the URL to the mint's icon
	// +optional
	IconURL string `json:"iconUrl,omitempty"`

	// ContactEmail is the contact email
	// +optional
	ContactEmail string `json:"contactEmail,omitempty"`

	// ContactNostrPubkey is the Nostr public key for contact
	// +optional
	ContactNostrPubkey string `json:"contactNostrPubkey,omitempty"`

	// TosURL is the URL to the terms of service
	// +optional
	TosURL string `json:"tosUrl,omitempty"`

	// InputFeePPK is the input fee in parts per thousand
	// +optional
	InputFeePPK *int32 `json:"inputFeePpk,omitempty"`

	// EnableSwaggerUI enables the Swagger UI
	// +optional
	EnableSwaggerUI bool `json:"enableSwaggerUi,omitempty"`
}

// DatabaseConfig specifies database configuration
type DatabaseConfig struct {
	// Engine specifies the database engine to use
	// +kubebuilder:validation:Enum=postgres;sqlite;redb
	// +kubebuilder:default=postgres
	Engine string `json:"engine"`

	// Postgres specifies PostgreSQL configuration
	// Required when engine is "postgres"
	// +optional
	Postgres *PostgresConfig `json:"postgres,omitempty"`

	// SQLite specifies SQLite configuration
	// Required when engine is "sqlite"
	// +optional
	SQLite *SQLiteConfig `json:"sqlite,omitempty"`
}

// PostgresConfig specifies PostgreSQL database configuration
type PostgresConfig struct {
	// URL is the PostgreSQL connection URL
	// Can be provided directly or via URLSecretRef
	// +optional
	URL string `json:"url,omitempty"`

	// URLSecretRef references a Secret containing the database URL
	// Preferred over direct URL for security
	// +optional
	URLSecretRef *corev1.SecretKeySelector `json:"urlSecretRef,omitempty"`

	// TLSMode specifies the TLS mode
	// +kubebuilder:validation:Enum=disable;prefer;require
	// +kubebuilder:default=require
	// +optional
	TLSMode string `json:"tlsMode,omitempty"`

	// MaxConnections is the maximum number of connections
	// +kubebuilder:default=20
	// +kubebuilder:validation:Minimum=1
	// +optional
	MaxConnections *int32 `json:"maxConnections,omitempty"`

	// ConnectionTimeoutSeconds is the connection timeout
	// +kubebuilder:default=10
	// +kubebuilder:validation:Minimum=1
	// +optional
	ConnectionTimeoutSeconds *int32 `json:"connectionTimeoutSeconds,omitempty"`

	// AutoProvision enables automatic PostgreSQL provisioning
	// If true, the operator will create a PostgreSQL instance
	// +optional
	AutoProvision bool `json:"autoProvision,omitempty"`

	// AutoProvisionSpec specifies auto-provisioning configuration
	// +optional
	AutoProvisionSpec *PostgresAutoProvisionSpec `json:"autoProvisionSpec,omitempty"`
}

// PostgresAutoProvisionSpec specifies auto-provisioning configuration
type PostgresAutoProvisionSpec struct {
	// StorageSize is the size of the PostgreSQL PVC
	// +kubebuilder:default="10Gi"
	StorageSize string `json:"storageSize,omitempty"`

	// StorageClassName is the storage class to use
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty"`

	// Resources specifies resource requirements
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// Version is the PostgreSQL version
	// +kubebuilder:default="15"
	Version string `json:"version,omitempty"`
}

// SQLiteConfig specifies SQLite database configuration
type SQLiteConfig struct {
	// DataDir is the directory where SQLite data is stored
	// +kubebuilder:default="/data"
	DataDir string `json:"dataDir,omitempty"`
}

// AuthDatabaseConfig specifies authentication database configuration
type AuthDatabaseConfig struct {
	// Postgres specifies PostgreSQL configuration for auth database
	// +optional
	Postgres *PostgresConfig `json:"postgres,omitempty"`
}

// LightningConfig specifies Lightning Network backend configuration
type LightningConfig struct {
	// Backend specifies which Lightning backend to use
	// +kubebuilder:validation:Enum=lnd;cln;lnbits;fakewallet;grpcprocessor
	Backend string `json:"backend"`

	// MinMint is the minimum amount for minting (in satoshis)
	// +optional
	MinMint *int64 `json:"minMint,omitempty"`

	// MaxMint is the maximum amount for minting (in satoshis)
	// +optional
	MaxMint *int64 `json:"maxMint,omitempty"`

	// MinMelt is the minimum amount for melting (in satoshis)
	// +optional
	MinMelt *int64 `json:"minMelt,omitempty"`

	// MaxMelt is the maximum amount for melting (in satoshis)
	// +optional
	MaxMelt *int64 `json:"maxMelt,omitempty"`

	// LND specifies LND backend configuration
	// Required when backend is "lnd"
	// +optional
	LND *LNDConfig `json:"lnd,omitempty"`

	// CLN specifies Core Lightning backend configuration
	// Required when backend is "cln"
	// +optional
	CLN *CLNConfig `json:"cln,omitempty"`

	// LNBits specifies LNBits backend configuration
	// Required when backend is "lnbits"
	// +optional
	LNBits *LNBitsConfig `json:"lnbits,omitempty"`

	// FakeWallet specifies fake wallet configuration (for testing)
	// Required when backend is "fakewallet"
	// +optional
	FakeWallet *FakeWalletConfig `json:"fakeWallet,omitempty"`

	// GRPCProcessor specifies gRPC processor configuration
	// Required when backend is "grpcprocessor"
	// +optional
	GRPCProcessor *GRPCProcessorConfig `json:"grpcProcessor,omitempty"`
}

// LNDConfig specifies LND backend configuration
type LNDConfig struct {
	// Address is the LND gRPC address
	// +kubebuilder:validation:Pattern=`^https?://.*:\d+$`
	Address string `json:"address"`

	// MacaroonSecretRef references a Secret containing the macaroon
	// +optional
	MacaroonSecretRef *corev1.SecretKeySelector `json:"macaroonSecretRef,omitempty"`

	// CertSecretRef references a Secret containing the TLS certificate
	// +optional
	CertSecretRef *corev1.SecretKeySelector `json:"certSecretRef,omitempty"`

	// FeePercent is the fee percentage
	// +kubebuilder:default=0.04
	// +optional
	FeePercent *float64 `json:"feePercent,omitempty"`

	// ReserveFeeMin is the minimum reserve fee
	// +kubebuilder:default=4
	// +optional
	ReserveFeeMin *int32 `json:"reserveFeeMin,omitempty"`
}

// CLNConfig specifies Core Lightning backend configuration
type CLNConfig struct {
	// RPCPath is the path to the CLN RPC socket
	RPCPath string `json:"rpcPath"`

	// FeePercent is the fee percentage
	// +kubebuilder:default=0.04
	// +optional
	FeePercent *float64 `json:"feePercent,omitempty"`

	// ReserveFeeMin is the minimum reserve fee
	// +kubebuilder:default=4
	// +optional
	ReserveFeeMin *int32 `json:"reserveFeeMin,omitempty"`
}

// LNBitsConfig specifies LNBits backend configuration
type LNBitsConfig struct {
	// API is the LNBits API URL
	// +kubebuilder:validation:Pattern=`^https?://.*`
	API string `json:"api"`

	// AdminAPIKeySecretRef references a Secret containing the admin API key
	AdminAPIKeySecretRef corev1.SecretKeySelector `json:"adminApiKeySecretRef"`

	// InvoiceAPIKeySecretRef references a Secret containing the invoice API key
	InvoiceAPIKeySecretRef corev1.SecretKeySelector `json:"invoiceApiKeySecretRef"`

	// RetroAPI enables backward compatibility with LNBits v0 API
	// +optional
	RetroAPI bool `json:"retroApi,omitempty"`
}

// FakeWalletConfig specifies fake wallet configuration for testing
type FakeWalletConfig struct {
	// SupportedUnits is the list of supported units
	// +kubebuilder:default={"sat"}
	SupportedUnits []string `json:"supportedUnits,omitempty"`

	// FeePercent is the fee percentage
	// +kubebuilder:default=0.02
	// +optional
	FeePercent *float64 `json:"feePercent,omitempty"`

	// ReserveFeeMin is the minimum reserve fee
	// +kubebuilder:default=1
	// +optional
	ReserveFeeMin *int32 `json:"reserveFeeMin,omitempty"`

	// MinDelayTime is the minimum delay time in seconds
	// +kubebuilder:default=1
	// +optional
	MinDelayTime *int32 `json:"minDelayTime,omitempty"`

	// MaxDelayTime is the maximum delay time in seconds
	// +kubebuilder:default=3
	// +optional
	MaxDelayTime *int32 `json:"maxDelayTime,omitempty"`
}

// GRPCProcessorConfig specifies gRPC payment processor configuration
type GRPCProcessorConfig struct {
	// Address is the gRPC processor address
	Address string `json:"address"`

	// Port is the gRPC processor port
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port"`

	// SupportedUnits is the list of supported units
	// +kubebuilder:default={"sat"}
	SupportedUnits []string `json:"supportedUnits,omitempty"`

	// TLSSecretRef references a Secret containing TLS certificates
	// If provided, the directory should contain client.crt, client.key, ca.crt
	// +optional
	TLSSecretRef *corev1.SecretKeySelector `json:"tlsSecretRef,omitempty"`
}

// LDKNodeConfig specifies LDK node configuration
type LDKNodeConfig struct {
	// Enabled controls whether LDK node is active
	// +kubebuilder:default=false
	Enabled bool `json:"enabled"`

	// FeePercent is the fee percentage
	// +kubebuilder:default=0.04
	// +optional
	FeePercent *float64 `json:"feePercent,omitempty"`

	// ReserveFeeMin is the minimum reserve fee
	// +kubebuilder:default=4
	// +optional
	ReserveFeeMin *int32 `json:"reserveFeeMin,omitempty"`

	// BitcoinNetwork specifies the Bitcoin network
	// +kubebuilder:validation:Enum=mainnet;testnet;signet;regtest
	// +kubebuilder:default=signet
	BitcoinNetwork string `json:"bitcoinNetwork,omitempty"`

	// ChainSourceType specifies the chain source
	// +kubebuilder:validation:Enum=esplora;bitcoinrpc
	// +kubebuilder:default=esplora
	ChainSourceType string `json:"chainSourceType,omitempty"`

	// EsploraURL is the Esplora API URL
	// +optional
	EsploraURL string `json:"esploraUrl,omitempty"`

	// BitcoinRPC specifies Bitcoin RPC configuration
	// +optional
	BitcoinRPC *BitcoinRPCConfig `json:"bitcoinRpc,omitempty"`

	// StorageDirPath is the storage directory path
	// Can be a local path or PostgreSQL URL
	// +optional
	StorageDirPath string `json:"storageDirPath,omitempty"`

	// Host is the LDK node listening host
	// +kubebuilder:default="0.0.0.0"
	// +optional
	Host string `json:"host,omitempty"`

	// Port is the LDK node listening port
	// +kubebuilder:default=8090
	// +optional
	Port int32 `json:"port,omitempty"`

	// AnnounceAddresses is the list of publicly announced addresses
	// +optional
	AnnounceAddresses []string `json:"announceAddresses,omitempty"`

	// GossipSourceType specifies the gossip source
	// +kubebuilder:validation:Enum=p2p;rgs
	// +kubebuilder:default=rgs
	// +optional
	GossipSourceType string `json:"gossipSourceType,omitempty"`

	// RGSURL is the RGS snapshot URL
	// +optional
	RGSURL string `json:"rgsUrl,omitempty"`

	// WebserverHost is the management webserver host
	// +kubebuilder:default="127.0.0.1"
	// +optional
	WebserverHost string `json:"webserverHost,omitempty"`

	// WebserverPort is the management webserver port
	// +kubebuilder:default=8888
	// +optional
	WebserverPort int32 `json:"webserverPort,omitempty"`
}

// BitcoinRPCConfig specifies Bitcoin RPC configuration
type BitcoinRPCConfig struct {
	// Host is the Bitcoin RPC host
	Host string `json:"host"`

	// Port is the Bitcoin RPC port
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port"`

	// UserSecretRef references a Secret containing the RPC username
	UserSecretRef corev1.SecretKeySelector `json:"userSecretRef"`

	// PasswordSecretRef references a Secret containing the RPC password
	PasswordSecretRef corev1.SecretKeySelector `json:"passwordSecretRef"`
}

// AuthConfig specifies authentication configuration
type AuthConfig struct {
	// Enabled controls whether authentication is active
	// +kubebuilder:default=false
	Enabled bool `json:"enabled"`

	// OpenIDDiscovery is the OpenID Connect discovery URL
	// +optional
	OpenIDDiscovery string `json:"openidDiscovery,omitempty"`

	// OpenIDClientID is the OpenID client ID
	// +optional
	OpenIDClientID string `json:"openidClientId,omitempty"`

	// MintMaxBat is the maximum batch size for mint operations
	// +kubebuilder:default=50
	// +optional
	MintMaxBat *int32 `json:"mintMaxBat,omitempty"`

	// EnabledMint controls whether authenticated minting is enabled
	// +kubebuilder:default=true
	// +optional
	EnabledMint *bool `json:"enabledMint,omitempty"`

	// EnabledMelt controls whether authenticated melting is enabled
	// +kubebuilder:default=true
	// +optional
	EnabledMelt *bool `json:"enabledMelt,omitempty"`

	// EnabledSwap controls whether authenticated swapping is enabled
	// +kubebuilder:default=true
	// +optional
	EnabledSwap *bool `json:"enabledSwap,omitempty"`

	// EnabledCheckMintQuote controls mint quote checking
	// +kubebuilder:default=true
	// +optional
	EnabledCheckMintQuote *bool `json:"enabledCheckMintQuote,omitempty"`

	// EnabledCheckMeltQuote controls melt quote checking
	// +kubebuilder:default=true
	// +optional
	EnabledCheckMeltQuote *bool `json:"enabledCheckMeltQuote,omitempty"`

	// EnabledRestore controls whether restore is enabled
	// +kubebuilder:default=true
	// +optional
	EnabledRestore *bool `json:"enabledRestore,omitempty"`

	// Database specifies authentication database configuration
	// +optional
	Database *AuthDatabaseConfig `json:"database,omitempty"`
}

// HTTPCacheConfig specifies HTTP cache configuration
type HTTPCacheConfig struct {
	// Backend specifies the cache backend
	// +kubebuilder:validation:Enum=memory;redis
	// +kubebuilder:default=memory
	Backend string `json:"backend"`

	// TTL is the time-to-live in seconds
	// +kubebuilder:default=60
	// +optional
	TTL *int32 `json:"ttl,omitempty"`

	// TTI is the time-to-idle in seconds
	// +kubebuilder:default=60
	// +optional
	TTI *int32 `json:"tti,omitempty"`

	// Redis specifies Redis configuration
	// Required when backend is "redis"
	// +optional
	Redis *RedisCacheConfig `json:"redis,omitempty"`
}

// RedisCacheConfig specifies Redis cache configuration
type RedisCacheConfig struct {
	// KeyPrefix is the Redis key prefix
	KeyPrefix string `json:"keyPrefix"`

	// ConnectionString is the Redis connection string
	// Can be provided directly or via ConnectionStringSecretRef
	// +optional
	ConnectionString string `json:"connectionString,omitempty"`

	// ConnectionStringSecretRef references a Secret containing the connection string
	// +optional
	ConnectionStringSecretRef *corev1.SecretKeySelector `json:"connectionStringSecretRef,omitempty"`
}

// ManagementRPCConfig specifies management RPC configuration
type ManagementRPCConfig struct {
	// Enabled controls whether management RPC is active
	// +kubebuilder:default=false
	Enabled bool `json:"enabled"`

	// Address is the listening address
	// +kubebuilder:default="127.0.0.1"
	// +optional
	Address string `json:"address,omitempty"`

	// Port is the listening port
	// +kubebuilder:default=8086
	// +optional
	Port int32 `json:"port,omitempty"`
}

// IngressConfig specifies ingress configuration
type IngressConfig struct {
	// Enabled controls whether Ingress is created
	// +kubebuilder:default=false
	Enabled bool `json:"enabled"`

	// ClassName is the ingress class name
	// +kubebuilder:default=nginx
	// +optional
	ClassName string `json:"className,omitempty"`

	// Host is the hostname for the ingress
	Host string `json:"host"`

	// TLS specifies TLS configuration
	// +optional
	TLS *IngressTLSConfig `json:"tls,omitempty"`

	// Annotations are additional annotations for the Ingress
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// IngressTLSConfig specifies ingress TLS configuration
type IngressTLSConfig struct {
	// Enabled controls whether TLS is enabled
	// +kubebuilder:default=true
	Enabled bool `json:"enabled"`

	// SecretName is the name of the TLS secret
	// If not provided and cert-manager is enabled, will be auto-generated
	// +optional
	SecretName string `json:"secretName,omitempty"`

	// CertManager specifies cert-manager configuration
	// +optional
	CertManager *CertManagerConfig `json:"certManager,omitempty"`
}

// CertManagerConfig specifies cert-manager configuration
type CertManagerConfig struct {
	// Enabled controls whether cert-manager integration is enabled
	// +kubebuilder:default=false
	Enabled bool `json:"enabled"`

	// IssuerName is the name of the cert-manager Issuer
	IssuerName string `json:"issuerName"`

	// IssuerKind is the kind of issuer (Issuer or ClusterIssuer)
	// +kubebuilder:validation:Enum=Issuer;ClusterIssuer
	// +kubebuilder:default=ClusterIssuer
	// +optional
	IssuerKind string `json:"issuerKind,omitempty"`
}

// ServiceConfig specifies service configuration
type ServiceConfig struct {
	// Type is the service type
	// +kubebuilder:validation:Enum=ClusterIP;NodePort;LoadBalancer
	// +kubebuilder:default=ClusterIP
	// +optional
	Type corev1.ServiceType `json:"type,omitempty"`

	// Annotations are additional annotations for the Service
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// LoadBalancerIP is the load balancer IP (for LoadBalancer type)
	// +optional
	LoadBalancerIP string `json:"loadBalancerIP,omitempty"`
}

// LoggingConfig specifies logging configuration
type LoggingConfig struct {
	// Level is the log level
	// +kubebuilder:validation:Enum=trace;debug;info;warn;error
	// +kubebuilder:default=info
	// +optional
	Level string `json:"level,omitempty"`

	// Format is the log format
	// +kubebuilder:validation:Enum=json;pretty
	// +kubebuilder:default=json
	// +optional
	Format string `json:"format,omitempty"`
}

// StorageConfig specifies persistent storage configuration
type StorageConfig struct {
	// Size is the storage size
	// +kubebuilder:default="10Gi"
	// +optional
	Size string `json:"size,omitempty"`

	// StorageClassName is the storage class name
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty"`
}

// CashuMintStatus defines the observed state of CashuMint
type CashuMintStatus struct {
	// Phase represents the current phase of the mint
	// +optional
	Phase MintPhase `json:"phase,omitempty"`

	// Conditions represent the latest available observations of the mint's state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation observed by the controller
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// DeploymentName is the name of the managed Deployment
	// +optional
	DeploymentName string `json:"deploymentName,omitempty"`

	// ServiceName is the name of the managed Service
	// +optional
	ServiceName string `json:"serviceName,omitempty"`

	// IngressName is the name of the managed Ingress
	// +optional
	IngressName string `json:"ingressName,omitempty"`

	// ConfigMapName is the name of the managed ConfigMap
	// +optional
	ConfigMapName string `json:"configMapName,omitempty"`

	// DatabaseStatus represents the database connection status
	// +optional
	DatabaseStatus DatabaseStatus `json:"databaseStatus,omitempty"`

	// LightningStatus represents the Lightning backend status
	// +optional
	LightningStatus LightningStatus `json:"lightningStatus,omitempty"`

	// URL is the actual URL where the mint is accessible
	// +optional
	URL string `json:"url,omitempty"`

	// ReadyReplicas is the number of ready replicas
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`
}

// MintPhase represents the deployment phase
// +kubebuilder:validation:Enum=Pending;Provisioning;Ready;Failed;Updating
type MintPhase string

const (
	MintPhasePending      MintPhase = "Pending"
	MintPhaseProvisioning MintPhase = "Provisioning"
	MintPhaseReady        MintPhase = "Ready"
	MintPhaseFailed       MintPhase = "Failed"
	MintPhaseUpdating     MintPhase = "Updating"
)

// DatabaseStatus represents database connection status
type DatabaseStatus struct {
	// Connected indicates whether the database is connected
	// +optional
	Connected bool `json:"connected,omitempty"`

	// Message provides additional information
	// +optional
	Message string `json:"message,omitempty"`

	// LastChecked is the timestamp of the last connectivity check
	// +optional
	LastChecked *metav1.Time `json:"lastChecked,omitempty"`
}

// LightningStatus represents Lightning backend status
type LightningStatus struct {
	// Connected indicates whether the Lightning backend is connected
	// +optional
	Connected bool `json:"connected,omitempty"`

	// Message provides additional information
	// +optional
	Message string `json:"message,omitempty"`

	// LastChecked is the timestamp of the last connectivity check
	// +optional
	LastChecked *metav1.Time `json:"lastChecked,omitempty"`
}

// Condition Types
const (
	// ConditionTypeReady indicates the mint is ready to serve requests
	ConditionTypeReady = "Ready"

	// ConditionTypeDatabaseReady indicates the database is ready
	ConditionTypeDatabaseReady = "DatabaseReady"

	// ConditionTypeLightningReady indicates the Lightning backend is ready
	ConditionTypeLightningReady = "LightningReady"

	// ConditionTypeConfigValid indicates the configuration is valid
	ConditionTypeConfigValid = "ConfigValid"

	// ConditionTypeIngressReady indicates the ingress is ready
	ConditionTypeIngressReady = "IngressReady"
)

// CashuMintList contains a list of CashuMint
// +kubebuilder:object:root=true
type CashuMintList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CashuMint `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CashuMint{}, &CashuMintList{})
}
