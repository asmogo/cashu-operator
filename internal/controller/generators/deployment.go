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
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	mintv1alpha1 "github.com/asmogo/cashu-operator/api/v1alpha1"
)

// GenerateDeployment creates a Deployment for the Cashu mint
func GenerateDeployment(mint *mintv1alpha1.CashuMint, configHash string, scheme *runtime.Scheme) (*appsv1.Deployment, error) {
	labels := map[string]string{
		"app.kubernetes.io/name":       "cashu-mint",
		"app.kubernetes.io/instance":   mint.Name,
		"app.kubernetes.io/managed-by": "cashu-operator",
	}

	replicas := int32(1)
	if mint.Spec.Replicas != nil {
		replicas = *mint.Spec.Replicas
	}

	// Pod annotations include config hash to trigger rolling updates on config changes
	podAnnotations := map[string]string{
		"config-hash": configHash,
	}

	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      mint.Name,
			Namespace: mint.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxUnavailable: &intstr.IntOrString{Type: intstr.Int, IntVal: 0},
					MaxSurge:       &intstr.IntOrString{Type: intstr.Int, IntVal: 1},
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: podAnnotations,
				},
				Spec: generatePodSpec(mint),
			},
		},
	}

	if err := controllerutil.SetControllerReference(mint, deployment, scheme); err != nil {
		return nil, fmt.Errorf("failed to set controller reference: %w", err)
	}

	return deployment, nil
}

// generatePodSpec creates the pod specification for the mint
func generatePodSpec(mint *mintv1alpha1.CashuMint) corev1.PodSpec {
	containers := []corev1.Container{}
	volumes := generateVolumes(mint)
	volumes = append(volumes, GenerateOrchardVolumes(mint)...)

	backend := mint.Spec.PaymentBackend.ActiveBackend()

	// Add generic gRPC processor sidecar if enabled
	if backend == mintv1alpha1.PaymentBackendGRPCProcessor &&
		mint.Spec.PaymentBackend.GRPCProcessor != nil &&
		mint.Spec.PaymentBackend.GRPCProcessor.SidecarProcessor != nil &&
		mint.Spec.PaymentBackend.GRPCProcessor.SidecarProcessor.Enabled {
		containers = append(containers, generateSidecarProcessorContainer(mint))
	}

	// Add LDK node sidecar if enabled
	if mint.Spec.LDKNode != nil && mint.Spec.LDKNode.Enabled {
		containers = append(containers, generateLDKContainer(mint))
	}
	containers = append(containers, generateMintContainer(mint))
	if orchardEnabled(mint) {
		containers = append(containers, GenerateOrchardContainer(mint))
	}
	podSpec := corev1.PodSpec{
		Containers:       containers,
		Volumes:          volumes,
		ImagePullSecrets: mint.Spec.ImagePullSecrets,
		NodeSelector:     mint.Spec.NodeSelector,
		Tolerations:      mint.Spec.Tolerations,
		Affinity:         mint.Spec.Affinity,
		SecurityContext:  getPodSecurityContext(mint),
	}

	return podSpec
}

// generateMintContainer creates the main mint container
func generateMintContainer(mint *mintv1alpha1.CashuMint) corev1.Container {
	image := mint.Spec.Image
	if image == "" {
		image = mintv1alpha1.DefaultMintImage
	}

	imagePullPolicy := mint.Spec.ImagePullPolicy
	if imagePullPolicy == "" {
		imagePullPolicy = corev1.PullIfNotPresent
	}

	listenPort := mint.Spec.MintInfo.ListenPort
	if listenPort == 0 {
		listenPort = 8085
	}

	ports := []corev1.ContainerPort{
		{
			Name:          "api",
			ContainerPort: listenPort,
			Protocol:      corev1.ProtocolTCP,
		},
	}

	// Add Prometheus metrics port if enabled
	if mint.Spec.Prometheus != nil && mint.Spec.Prometheus.Enabled {
		metricsPort := int32(9090)
		if mint.Spec.Prometheus.Port != nil {
			metricsPort = *mint.Spec.Prometheus.Port
		}
		ports = append(ports, corev1.ContainerPort{
			Name:          "metrics",
			ContainerPort: metricsPort,
			Protocol:      corev1.ProtocolTCP,
		})
	}

	container := corev1.Container{
		Name:            "mintd",
		Image:           image,
		ImagePullPolicy: imagePullPolicy,
		Ports:           ports,
		Env:             generateEnvironmentVariables(mint),
		VolumeMounts:    generateVolumeMounts(mint),
		Resources:       getResourceRequirements(mint),
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   "/v1/info",
					Port:   intstr.FromString("api"),
					Scheme: corev1.URISchemeHTTP,
				},
			},
			InitialDelaySeconds: 30,
			PeriodSeconds:       30,
			TimeoutSeconds:      10,
			SuccessThreshold:    1,
			FailureThreshold:    3,
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   "/v1/info",
					Port:   intstr.FromString("api"),
					Scheme: corev1.URISchemeHTTP,
				},
			},
			InitialDelaySeconds: 10,
			PeriodSeconds:       10,
			TimeoutSeconds:      5,
			SuccessThreshold:    1,
			FailureThreshold:    3,
		},
		SecurityContext: getContainerSecurityContext(mint),
	}

	return container
}

// generateLDKContainer creates the LDK node sidecar container
func generateLDKContainer(mint *mintv1alpha1.CashuMint) corev1.Container {
	ldkPort := mint.Spec.LDKNode.Port
	if ldkPort == 0 {
		ldkPort = 8090
	}

	webserverPort := mint.Spec.LDKNode.WebserverPort
	if webserverPort == 0 {
		webserverPort = 8888
	}

	image := mint.Spec.LDKNode.Image
	if image == "" {
		image = "ghcr.io/cashubtc/ldk-node:latest"
	}

	return corev1.Container{
		Name:  "ldk-node",
		Image: image,
		Ports: []corev1.ContainerPort{
			{
				Name:          "ldk",
				ContainerPort: ldkPort,
				Protocol:      corev1.ProtocolTCP,
			},
			{
				Name:          "webserver",
				ContainerPort: webserverPort,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "data",
				MountPath: "/data",
			},
		},
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: boolPtr(false),
			RunAsNonRoot:             boolPtr(true),
		},
	}
}

// generateSidecarProcessorContainer creates a generic gRPC payment processor sidecar container.
// It supports any processor image configured via spec.paymentBackend.grpcProcessor.sidecarProcessor.
func generateSidecarProcessorContainer(mint *mintv1alpha1.CashuMint) corev1.Container {
	sidecarConfig := mint.Spec.PaymentBackend.GRPCProcessor.SidecarProcessor

	imagePullPolicy := sidecarConfig.ImagePullPolicy
	if imagePullPolicy == "" {
		imagePullPolicy = corev1.PullIfNotPresent
	}

	port := mint.Spec.PaymentBackend.GRPCProcessor.Port
	if port == 0 {
		port = 50051
	}

	// Build volume mounts - shared data volume and optional TLS
	volumeMounts := []corev1.VolumeMount{}
	if sidecarConfig.WorkingDir != "" {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "data",
			MountPath: sidecarConfig.WorkingDir,
			SubPath:   "sidecar-processor",
		})
	}

	// Add TLS secret volume mount if needed
	if sidecarConfig.EnableTLS && sidecarConfig.TLSSecretRef != nil {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "sidecar-tls",
			MountPath: "/secrets/sidecar-tls",
			ReadOnly:  true,
		})
	}

	// Build container - the user provides all env vars, command, and args
	container := corev1.Container{
		Name:            "grpc-processor",
		Image:           sidecarConfig.Image,
		ImagePullPolicy: imagePullPolicy,
		Command:         sidecarConfig.Command,
		Args:            sidecarConfig.Args,
		Ports: []corev1.ContainerPort{
			{
				Name:          "grpc",
				ContainerPort: port,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		Env:          sidecarConfig.Env,
		VolumeMounts: volumeMounts,
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: boolPtr(false),
		},
	}

	// Add resource requirements if specified
	if sidecarConfig.Resources != nil {
		container.Resources = *sidecarConfig.Resources
	}

	return container
}

// generateEnvironmentVariables creates environment variables for the mint container
func generateEnvironmentVariables(mint *mintv1alpha1.CashuMint) []corev1.EnvVar {
	backend := mint.Spec.PaymentBackend.ActiveBackend()

	listenHost := mint.Spec.MintInfo.ListenHost
	if listenHost == "" {
		listenHost = mintv1alpha1.DefaultListenHost
	}
	listenPort := mint.Spec.MintInfo.ListenPort
	if listenPort == 0 {
		listenPort = 8085
	}

	// cdk-mintd >= 0.15.0 reads critical settings exclusively from env vars when
	// Settings::from_env() is called after config file parsing. We set all the
	// non-secret fields as env vars so the mint starts correctly even if config
	// file parsing fails silently.
	envVars := []corev1.EnvVar{
		{
			// CDK_MINTD_WORK_DIR: work directory — binary looks for config.toml here
			// and stores SQLite DB, logs, TLS material under this path.
			Name:  "CDK_MINTD_WORK_DIR",
			Value: "/data",
		},
		{
			Name:  "HOME",
			Value: "/data",
		},
		{
			// CDK_MINTD_LN_BACKEND: canonical backend selector in 0.15.0+
			Name:  "CDK_MINTD_LN_BACKEND",
			Value: backend,
		},
		{
			// CDK_MINTD_URL: public mint URL
			Name:  "CDK_MINTD_URL",
			Value: mint.Spec.MintInfo.URL,
		},
		{
			// CDK_MINTD_LISTEN_HOST: bind address for the HTTP server
			Name:  "CDK_MINTD_LISTEN_HOST",
			Value: listenHost,
		},
		{
			// CDK_MINTD_LISTEN_PORT: bind port for the HTTP server
			Name:  "CDK_MINTD_LISTEN_PORT",
			Value: fmt.Sprintf("%d", listenPort),
		},
		{
			// CDK_MINTD_DATABASE: database engine ("sqlite" or "postgres")
			Name:  "CDK_MINTD_DATABASE",
			Value: mint.Spec.Database.Engine,
		},
		{
			// CDK_MINTD_LOGGING_OUTPUT=stderr activates the tracing subscriber
			// without needing to pass --enable-logging as a CLI arg (which would
			// override the container entrypoint).
			Name:  "CDK_MINTD_LOGGING_OUTPUT",
			Value: "stderr",
		},
	}

	// Logging configuration
	if mint.Spec.Logging != nil {
		if mint.Spec.Logging.Level != "" {
			envVars = append(envVars, corev1.EnvVar{
				Name:  "RUST_LOG",
				Value: mint.Spec.Logging.Level,
			})
			envVars = append(envVars, corev1.EnvVar{
				Name:  "CDK_MINTD_LOGGING_CONSOLE_LEVEL",
				Value: mint.Spec.Logging.Level,
			})
		}
		if mint.Spec.Logging.FileLevel != "" {
			envVars = append(envVars, corev1.EnvVar{
				Name:  "CDK_MINTD_LOGGING_FILE_LEVEL",
				Value: mint.Spec.Logging.FileLevel,
			})
		}
	}

	// Mnemonic from secret — either user-provided ref or auto-generated secret.
	mnemonicRef := mint.Spec.MintInfo.MnemonicSecretRef
	if mnemonicRef == nil && mint.Spec.MintInfo.AutoGenerateMnemonic {
		mnemonicRef = &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: MnemonicSecretName(mint.Name)},
			Key:                  MnemonicSecretKey,
		}
	}
	if mnemonicRef != nil {
		envVars = append(envVars, corev1.EnvVar{
			Name: "CDK_MINTD_MNEMONIC",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: mnemonicRef,
			},
		})
	}

	// Database URL — injected for auto-provisioned postgres via the operator-managed
	// secret, and for external postgres via user-supplied secret or plain URL.
	if mint.Spec.Database.Engine == mintv1alpha1.DatabaseEnginePostgres && mint.Spec.Database.Postgres != nil {
		if mint.Spec.Database.Postgres.AutoProvision {
			// Auto-provisioned: read database-url key from the operator-managed secret
			envVars = append(envVars, corev1.EnvVar{
				Name: "CDK_MINTD_POSTGRES_URL",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: mint.Name + "-postgres-secret",
						},
						Key: "database-url",
					},
				},
			})
		} else if mint.Spec.Database.Postgres.URLSecretRef != nil {
			envVars = append(envVars, corev1.EnvVar{
				Name: "CDK_MINTD_POSTGRES_URL",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: mint.Spec.Database.Postgres.URLSecretRef,
				},
			})
		} else if mint.Spec.Database.Postgres.URL != "" {
			envVars = append(envVars, corev1.EnvVar{
				Name:  "CDK_MINTD_POSTGRES_URL",
				Value: mint.Spec.Database.Postgres.URL,
			})
		}
	}

	// Auth database configuration
	if mint.Spec.Auth != nil && mint.Spec.Auth.Enabled &&
		mint.Spec.Auth.Database != nil && mint.Spec.Auth.Database.Postgres != nil {
		if mint.Spec.Auth.Database.Postgres.URLSecretRef != nil {
			envVars = append(envVars, corev1.EnvVar{
				Name: "CDK_MINTD_AUTH_POSTGRES_URL",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: mint.Spec.Auth.Database.Postgres.URLSecretRef,
				},
			})
		}
	}

	// LNBits API keys from secrets
	if backend == mintv1alpha1.PaymentBackendLNBits && mint.Spec.PaymentBackend.LNBits != nil {
		envVars = append(envVars, corev1.EnvVar{
			Name: "LNBITS_ADMIN_API_KEY",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &mint.Spec.PaymentBackend.LNBits.AdminAPIKeySecretRef,
			},
		})
		envVars = append(envVars, corev1.EnvVar{
			Name: "LNBITS_INVOICE_API_KEY",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &mint.Spec.PaymentBackend.LNBits.InvoiceAPIKeySecretRef,
			},
		})
	}

	// Bitcoin RPC credentials for LDK node
	if mint.Spec.LDKNode != nil && mint.Spec.LDKNode.Enabled &&
		mint.Spec.LDKNode.BitcoinRPC != nil {
		envVars = append(envVars, corev1.EnvVar{
			Name: "BITCOIN_RPC_USER",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &mint.Spec.LDKNode.BitcoinRPC.UserSecretRef,
			},
		})
		envVars = append(envVars, corev1.EnvVar{
			Name: "BITCOIN_RPC_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &mint.Spec.LDKNode.BitcoinRPC.PasswordSecretRef,
			},
		})
	}

	// LDK node mnemonic from secret
	if mint.Spec.LDKNode != nil && mint.Spec.LDKNode.Enabled &&
		mint.Spec.LDKNode.MnemonicSecretRef != nil {
		envVars = append(envVars, corev1.EnvVar{
			Name: "CDK_LDK_NODE_MNEMONIC",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: mint.Spec.LDKNode.MnemonicSecretRef,
			},
		})
	}

	// HTTP cache Redis connection string
	if mint.Spec.HTTPCache != nil && mint.Spec.HTTPCache.Backend == "redis" &&
		mint.Spec.HTTPCache.Redis != nil {
		if mint.Spec.HTTPCache.Redis.ConnectionStringSecretRef != nil {
			envVars = append(envVars, corev1.EnvVar{
				Name: "REDIS_CONNECTION_STRING",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: mint.Spec.HTTPCache.Redis.ConnectionStringSecretRef,
				},
			})
		} else if mint.Spec.HTTPCache.Redis.ConnectionString != "" {
			envVars = append(envVars, corev1.EnvVar{
				Name:  "REDIS_CONNECTION_STRING",
				Value: mint.Spec.HTTPCache.Redis.ConnectionString,
			})
		}
	}

	return envVars
}

// generateVolumeMounts creates volume mounts for the mint container
func generateVolumeMounts(mint *mintv1alpha1.CashuMint) []corev1.VolumeMount {
	// IMPORTANT: "data" must be mounted before "config" so that the subPath
	// config.toml bind-mount is applied into the already-present /data directory.
	// If "config" came first, the subsequent emptyDir/PVC mount of /data would
	// shadow the subPath file and CDK would start with no config.toml.
	mounts := []corev1.VolumeMount{
		{
			Name:      "data",
			MountPath: "/data",
		},
		{
			// Mount config.toml into the work dir so CDK finds it at $CDK_MINTD_WORK_DIR/config.toml
			Name:      "config",
			MountPath: "/data/config.toml",
			SubPath:   "config.toml",
			ReadOnly:  true,
		},
	}

	backend := mint.Spec.PaymentBackend.ActiveBackend()

	// LND macaroon and cert
	if backend == mintv1alpha1.PaymentBackendLND && mint.Spec.PaymentBackend.LND != nil {
		if mint.Spec.PaymentBackend.LND.MacaroonSecretRef != nil {
			mounts = append(mounts, corev1.VolumeMount{
				Name:      "lnd-macaroon",
				MountPath: "/secrets/lnd/macaroon",
				SubPath:   mint.Spec.PaymentBackend.LND.MacaroonSecretRef.Key,
				ReadOnly:  true,
			})
		}
		if mint.Spec.PaymentBackend.LND.CertSecretRef != nil {
			mounts = append(mounts, corev1.VolumeMount{
				Name:      "lnd-cert",
				MountPath: "/secrets/lnd/cert",
				SubPath:   mint.Spec.PaymentBackend.LND.CertSecretRef.Key,
				ReadOnly:  true,
			})
		}
	}

	// gRPC processor TLS certificates
	if backend == mintv1alpha1.PaymentBackendGRPCProcessor &&
		mint.Spec.PaymentBackend.GRPCProcessor != nil &&
		mint.Spec.PaymentBackend.GRPCProcessor.TLSSecretRef != nil {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      "grpc-tls",
			MountPath: "/secrets/grpc",
			ReadOnly:  true,
		})
	}

	if mintv1alpha1.ManagementRPCTLSEnabled(&mint.Spec) {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      managementRPCTLSVolumeName,
			MountPath: orchardManagementRPCTLSMountPath,
			ReadOnly:  true,
		})
	}

	return mounts
}

// generateVolumes creates volumes for the pod
func generateVolumes(mint *mintv1alpha1.CashuMint) []corev1.Volume {
	volumes := []corev1.Volume{
		{
			Name: "config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: mint.Name + "-config",
					},
				},
			},
		},
	}

	// Data volume - either PVC or emptyDir
	if mint.Spec.Database.Engine == mintv1alpha1.DatabaseEngineSQLite || mint.Spec.Database.Engine == mintv1alpha1.DatabaseEngineRedb {
		// Use PVC for SQLite/redb
		volumes = append(volumes, corev1.Volume{
			Name: "data",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: mint.Name + "-data",
				},
			},
		})
	} else {
		// Use emptyDir for PostgreSQL (data in external DB)
		volumes = append(volumes, corev1.Volume{
			Name: "data",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})
	}

	backend := mint.Spec.PaymentBackend.ActiveBackend()

	// LND secret volumes
	if backend == mintv1alpha1.PaymentBackendLND && mint.Spec.PaymentBackend.LND != nil {
		if mint.Spec.PaymentBackend.LND.MacaroonSecretRef != nil {
			volumes = append(volumes, corev1.Volume{
				Name: "lnd-macaroon",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: mint.Spec.PaymentBackend.LND.MacaroonSecretRef.Name,
					},
				},
			})
		}
		if mint.Spec.PaymentBackend.LND.CertSecretRef != nil {
			volumes = append(volumes, corev1.Volume{
				Name: "lnd-cert",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: mint.Spec.PaymentBackend.LND.CertSecretRef.Name,
					},
				},
			})
		}
	}

	// gRPC processor TLS volume
	if backend == mintv1alpha1.PaymentBackendGRPCProcessor &&
		mint.Spec.PaymentBackend.GRPCProcessor != nil &&
		mint.Spec.PaymentBackend.GRPCProcessor.TLSSecretRef != nil {
		volumes = append(volumes, corev1.Volume{
			Name: "grpc-tls",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: mint.Spec.PaymentBackend.GRPCProcessor.TLSSecretRef.Name,
				},
			},
		})
	}

	if mintv1alpha1.ManagementRPCTLSEnabled(&mint.Spec) {
		volumes = append(volumes, corev1.Volume{
			Name: managementRPCTLSVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: mintv1alpha1.ManagementRPCTLSSecretName(&mint.Spec, mint.Name),
				},
			},
		})
	}

	// Sidecar processor TLS volume
	if backend == mintv1alpha1.PaymentBackendGRPCProcessor &&
		mint.Spec.PaymentBackend.GRPCProcessor != nil &&
		mint.Spec.PaymentBackend.GRPCProcessor.SidecarProcessor != nil &&
		mint.Spec.PaymentBackend.GRPCProcessor.SidecarProcessor.Enabled &&
		mint.Spec.PaymentBackend.GRPCProcessor.SidecarProcessor.EnableTLS &&
		mint.Spec.PaymentBackend.GRPCProcessor.SidecarProcessor.TLSSecretRef != nil {
		volumes = append(volumes, corev1.Volume{
			Name: "sidecar-tls",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: mint.Spec.PaymentBackend.GRPCProcessor.SidecarProcessor.TLSSecretRef.Name,
				},
			},
		})
	}

	return volumes
}

// getResourceRequirements returns resource requirements
func getResourceRequirements(mint *mintv1alpha1.CashuMint) corev1.ResourceRequirements {
	if mint.Spec.Resources != nil {
		return *mint.Spec.Resources
	}

	// Default resource requirements
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("100m"),
			corev1.ResourceMemory: resource.MustParse("128Mi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1000m"),
			corev1.ResourceMemory: resource.MustParse("512Mi"),
		},
	}
}

// getPodSecurityContext returns the pod security context
func getPodSecurityContext(mint *mintv1alpha1.CashuMint) *corev1.PodSecurityContext {
	if mint.Spec.PodSecurityContext != nil {
		return mint.Spec.PodSecurityContext
	}

	// Default security context
	return &corev1.PodSecurityContext{
		RunAsNonRoot: boolPtr(true),
		RunAsUser:    int64Ptr(1000),
		FSGroup:      int64Ptr(1000),
	}
}

// getContainerSecurityContext returns the container security context
func getContainerSecurityContext(mint *mintv1alpha1.CashuMint) *corev1.SecurityContext {
	if mint.Spec.ContainerSecurityContext != nil {
		return mint.Spec.ContainerSecurityContext
	}

	// Default security context
	return &corev1.SecurityContext{
		AllowPrivilegeEscalation: boolPtr(false),
		ReadOnlyRootFilesystem:   boolPtr(false),
		RunAsNonRoot:             boolPtr(true),
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		},
	}
}

// Helper functions
func boolPtr(b bool) *bool {
	return &b
}

func int64Ptr(i int64) *int64 {
	return &i
}
