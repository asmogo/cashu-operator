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
	containers := []corev1.Container{
		generateMintContainer(mint),
	}

	// Add LDK node sidecar if enabled
	if mint.Spec.LDKNode != nil && mint.Spec.LDKNode.Enabled {
		containers = append(containers, generateLDKContainer(mint))
	}

	podSpec := corev1.PodSpec{
		Containers:       containers,
		Volumes:          generateVolumes(mint),
		ImagePullSecrets: mint.Spec.ImagePullSecrets,
		NodeSelector:     mint.Spec.NodeSelector,
		Tolerations:      mint.Spec.Tolerations,
		Affinity:         mint.Spec.Affinity,
		SecurityContext: &corev1.PodSecurityContext{
			RunAsNonRoot: boolPtr(true),
			RunAsUser:    int64Ptr(1000),
			FSGroup:      int64Ptr(1000),
		},
	}

	return podSpec
}

// generateMintContainer creates the main mint container
func generateMintContainer(mint *mintv1alpha1.CashuMint) corev1.Container {
	image := mint.Spec.Image
	if image == "" {
		image = "cashubtc/mintd:latest"
	}

	imagePullPolicy := mint.Spec.ImagePullPolicy
	if imagePullPolicy == "" {
		imagePullPolicy = corev1.PullIfNotPresent
	}

	listenPort := mint.Spec.MintInfo.ListenPort
	if listenPort == 0 {
		listenPort = 8085
	}

	container := corev1.Container{
		Name:            "mintd",
		Image:           image,
		ImagePullPolicy: imagePullPolicy,
		Ports: []corev1.ContainerPort{
			{
				Name:          "api",
				ContainerPort: listenPort,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		Env:          generateEnvironmentVariables(mint),
		VolumeMounts: generateVolumeMounts(mint),
		Resources:    getResourceRequirements(mint),
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   "/health",
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
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: boolPtr(false),
			ReadOnlyRootFilesystem:   boolPtr(false),
			RunAsNonRoot:             boolPtr(true),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"ALL"},
			},
		},
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

	return corev1.Container{
		Name:  "ldk-node",
		Image: "ghcr.io/cashubtc/ldk-node:latest", // TODO: Make this configurable
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

// generateEnvironmentVariables creates environment variables for the mint container
func generateEnvironmentVariables(mint *mintv1alpha1.CashuMint) []corev1.EnvVar {
	envVars := []corev1.EnvVar{
		{
			Name:  "CASHU_CONFIG",
			Value: "/config/config.toml",
		},
		{
			Name:  "CASHU_DATA_DIR",
			Value: "/data",
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
				Name:  "LOG_LEVEL",
				Value: mint.Spec.Logging.Level,
			})
		}
		if mint.Spec.Logging.Format != "" {
			envVars = append(envVars, corev1.EnvVar{
				Name:  "LOG_FORMAT",
				Value: mint.Spec.Logging.Format,
			})
		}
	}

	// Mnemonic from secret
	if mint.Spec.MintInfo.MnemonicSecretRef != nil {
		envVars = append(envVars, corev1.EnvVar{
			Name: "CASHU_MNEMONIC",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: mint.Spec.MintInfo.MnemonicSecretRef,
			},
		})
	}

	// Database configuration
	if mint.Spec.Database.Engine == "postgres" && mint.Spec.Database.Postgres != nil {
		// Database URL from secret or auto-provisioned
		if mint.Spec.Database.Postgres.URLSecretRef != nil {
			envVars = append(envVars, corev1.EnvVar{
				Name: "CDK_MINTD_DATABASE_URL",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: mint.Spec.Database.Postgres.URLSecretRef,
				},
			})
		} else if mint.Spec.Database.Postgres.AutoProvision {
			// Auto-provisioned PostgreSQL
			envVars = append(envVars, corev1.EnvVar{
				Name: "CDK_MINTD_DATABASE_URL",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: mint.Name + "-postgres-secret",
						},
						Key: "database-url",
					},
				},
			})
		} else if mint.Spec.Database.Postgres.URL != "" {
			// Direct URL (not recommended for production)
			envVars = append(envVars, corev1.EnvVar{
				Name:  "CDK_MINTD_DATABASE_URL",
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
	if mint.Spec.Lightning.Backend == "lnbits" && mint.Spec.Lightning.LNBits != nil {
		envVars = append(envVars, corev1.EnvVar{
			Name: "LNBITS_ADMIN_API_KEY",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &mint.Spec.Lightning.LNBits.AdminAPIKeySecretRef,
			},
		})
		envVars = append(envVars, corev1.EnvVar{
			Name: "LNBITS_INVOICE_API_KEY",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &mint.Spec.Lightning.LNBits.InvoiceAPIKeySecretRef,
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
	mounts := []corev1.VolumeMount{
		{
			Name:      "config",
			MountPath: "/config/config.toml",
			SubPath:   "config.toml",
			ReadOnly:  true,
		},
		{
			Name:      "data",
			MountPath: "/data",
		},
	}

	// LND macaroon and cert
	if mint.Spec.Lightning.Backend == "lnd" && mint.Spec.Lightning.LND != nil {
		if mint.Spec.Lightning.LND.MacaroonSecretRef != nil {
			mounts = append(mounts, corev1.VolumeMount{
				Name:      "lnd-macaroon",
				MountPath: "/secrets/lnd/macaroon",
				SubPath:   mint.Spec.Lightning.LND.MacaroonSecretRef.Key,
				ReadOnly:  true,
			})
		}
		if mint.Spec.Lightning.LND.CertSecretRef != nil {
			mounts = append(mounts, corev1.VolumeMount{
				Name:      "lnd-cert",
				MountPath: "/secrets/lnd/cert",
				SubPath:   mint.Spec.Lightning.LND.CertSecretRef.Key,
				ReadOnly:  true,
			})
		}
	}

	// gRPC processor TLS certificates
	if mint.Spec.Lightning.Backend == "grpcprocessor" &&
		mint.Spec.Lightning.GRPCProcessor != nil &&
		mint.Spec.Lightning.GRPCProcessor.TLSSecretRef != nil {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      "grpc-tls",
			MountPath: "/secrets/grpc",
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
	if mint.Spec.Database.Engine == "sqlite" || mint.Spec.Database.Engine == "redb" {
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

	// LND secret volumes
	if mint.Spec.Lightning.Backend == "lnd" && mint.Spec.Lightning.LND != nil {
		if mint.Spec.Lightning.LND.MacaroonSecretRef != nil {
			volumes = append(volumes, corev1.Volume{
				Name: "lnd-macaroon",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: mint.Spec.Lightning.LND.MacaroonSecretRef.Name,
					},
				},
			})
		}
		if mint.Spec.Lightning.LND.CertSecretRef != nil {
			volumes = append(volumes, corev1.Volume{
				Name: "lnd-cert",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: mint.Spec.Lightning.LND.CertSecretRef.Name,
					},
				},
			})
		}
	}

	// gRPC processor TLS volume
	if mint.Spec.Lightning.Backend == "grpcprocessor" &&
		mint.Spec.Lightning.GRPCProcessor != nil &&
		mint.Spec.Lightning.GRPCProcessor.TLSSecretRef != nil {
		volumes = append(volumes, corev1.Volume{
			Name: "grpc-tls",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: mint.Spec.Lightning.GRPCProcessor.TLSSecretRef.Name,
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

// Helper functions
func boolPtr(b bool) *bool {
	return &b
}

func int64Ptr(i int64) *int64 {
	return &i
}
