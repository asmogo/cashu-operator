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
	"strconv"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	mintv1alpha1 "github.com/asmogo/cashu-operator/api/v1alpha1"
)

const (
	orchardPortDefault                 int32 = 3321
	orchardDataDir                           = "/app/data"
	orchardTmpDir                            = "/app/data/tmp"
	orchardMintDataDir                       = "/mnt/mint"
	orchardMintSQLitePath                    = "/mnt/mint/cdk-mintd.sqlite"
	orchardManagementRPCTLSMountPath         = "/secrets/management-rpc-tls"
	orchardLightningMacaroonPath             = "/secrets/orchard/lightning/macaroon"
	orchardLightningCertPath                 = "/secrets/orchard/lightning/cert"
	orchardLightningKeyPath                  = "/secrets/orchard/lightning/key"
	orchardLightningCAPath                   = "/secrets/orchard/lightning/ca"
	orchardTaprootMacaroonPath               = "/secrets/orchard/taproot/macaroon"
	orchardTaprootCertPath                   = "/secrets/orchard/taproot/cert"
	orchardDataVolumeName                    = "orchard-data"
	orchardTmpVolumeName                     = "orchard-tmp"
	orchardLightningMacaroonVolumeName       = "orchard-lightning-macaroon"
	orchardLightningCertVolumeName           = "orchard-lightning-cert"
	orchardLightningKeyVolumeName            = "orchard-lightning-key"
	orchardLightningCAVolumeName             = "orchard-lightning-ca"
	orchardTaprootMacaroonVolumeName         = "orchard-taproot-macaroon"
	orchardTaprootCertVolumeName             = "orchard-taproot-cert"
)

func orchardEnabled(mint *mintv1alpha1.CashuMint) bool {
	return mint.Spec.Orchard != nil && mint.Spec.Orchard.Enabled
}

func orchardResourceName(mint *mintv1alpha1.CashuMint) string {
	return mint.Name + "-orchard"
}

func orchardPVCName(mint *mintv1alpha1.CashuMint) string {
	return orchardResourceName(mint) + "-data"
}

func mintSelectorLabels(mint *mintv1alpha1.CashuMint) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "cashu-mint",
		"app.kubernetes.io/instance":   mint.Name,
		"app.kubernetes.io/managed-by": "cashu-operator",
	}
}

func orchardLabels(mint *mintv1alpha1.CashuMint) map[string]string {
	labels := mintSelectorLabels(mint)
	labels["app.kubernetes.io/component"] = orchardStr
	return labels
}

// GenerateOrchardContainer creates the Orchard companion container.
func GenerateOrchardContainer(mint *mintv1alpha1.CashuMint) corev1.Container {
	orchard := mint.Spec.Orchard
	image := orchard.Image
	if image == "" {
		image = mintv1alpha1.DefaultOrchardImage(mint.Spec.Database.Engine)
	}

	imagePullPolicy := orchard.ImagePullPolicy
	if imagePullPolicy == "" {
		imagePullPolicy = corev1.PullIfNotPresent
	}

	port := orchard.Port
	if port == 0 {
		port = orchardPortDefault
	}

	return corev1.Container{
		Name:            orchardStr,
		Image:           image,
		ImagePullPolicy: imagePullPolicy,
		Ports: []corev1.ContainerPort{
			{
				Name:          orchardStr,
				ContainerPort: port,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		Env:          generateOrchardEnvironmentVariables(mint),
		VolumeMounts: generateOrchardVolumeMounts(mint),
		Resources:    getOrchardResourceRequirements(mint),
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromString(orchardStr)},
			},
			InitialDelaySeconds: 30,
			PeriodSeconds:       30,
			TimeoutSeconds:      5,
			FailureThreshold:    3,
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromString(orchardStr)},
			},
			InitialDelaySeconds: 10,
			PeriodSeconds:       10,
			TimeoutSeconds:      5,
			FailureThreshold:    3,
		},
		SecurityContext: getOrchardContainerSecurityContext(mint),
	}
}

func generateOrchardEnvironmentVariables(mint *mintv1alpha1.CashuMint) []corev1.EnvVar {
	orchard := mint.Spec.Orchard
	envVars := []corev1.EnvVar{
		{Name: "NODE_ENV", Value: "production"},
		{Name: "SERVER_HOST", Value: orchard.Host},
		{Name: "SERVER_PORT", Value: strconv.Itoa(int(orchardPort(mint)))},
		{Name: "BASE_PATH", Value: orchard.BasePath},
		{Name: "LOG_LEVEL", Value: orchard.LogLevel},
		{Name: "DATABASE_DIR", Value: orchardDataDir},
		{Name: "MINT_TYPE", Value: orchardMintType(mint)},
		{Name: "MINT_API", Value: orchardMintAPI(mint)},
		{Name: "MINT_RPC_HOST", Value: orchardMintRPCHost(mint)},
		{Name: "MINT_RPC_PORT", Value: strconv.Itoa(int(orchardMintRPCPort(mint)))},
		{Name: "MINT_RPC_MTLS", Value: strconv.FormatBool(orchardMintRPCMTLS(mint))},
		{
			Name: "SETUP_KEY",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: orchard.SetupKeySecretRef,
			},
		},
	}

	if orchard.ThrottleTTL != nil {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "THROTTLE_TTL",
			Value: strconv.Itoa(int(*orchard.ThrottleTTL)),
		})
	}
	if orchard.ThrottleLimit != nil {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "THROTTLE_LIMIT",
			Value: strconv.Itoa(int(*orchard.ThrottleLimit)),
		})
	}
	if orchard.Proxy != "" {
		envVars = append(envVars, corev1.EnvVar{Name: "TOR_PROXY_SERVER", Value: orchard.Proxy})
	}
	if orchard.Compression != nil {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "SERVER_COMPRESSION",
			Value: strconv.FormatBool(*orchard.Compression),
		})
	}

	if mintDBEnv := generateOrchardMintDatabaseEnvVar(mint); mintDBEnv != nil {
		envVars = append(envVars, *mintDBEnv)
	}

	if orchardSpecifiesMintDatabaseTLS(orchard) {
		if orchard.Mint.DatabaseCASecretRef != nil {
			envVars = append(envVars, corev1.EnvVar{
				Name: "MINT_DATABASE_CA",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: orchard.Mint.DatabaseCASecretRef,
				},
			})
		}
		if orchard.Mint.DatabaseCertSecretRef != nil {
			envVars = append(envVars, corev1.EnvVar{
				Name: "MINT_DATABASE_CERT",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: orchard.Mint.DatabaseCertSecretRef,
				},
			})
		}
		if orchard.Mint.DatabaseKeySecretRef != nil {
			envVars = append(envVars, corev1.EnvVar{
				Name: "MINT_DATABASE_KEY",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: orchard.Mint.DatabaseKeySecretRef,
				},
			})
		}
	}

	if orchardMintRPCMTLS(mint) {
		envVars = append(envVars,
			corev1.EnvVar{Name: "MINT_RPC_KEY", Value: orchardManagementRPCTLSMountPath + "/client.key"},
			corev1.EnvVar{Name: "MINT_RPC_CERT", Value: orchardManagementRPCTLSMountPath + "/client.pem"},
			corev1.EnvVar{Name: "MINT_RPC_CA", Value: orchardManagementRPCTLSMountPath + "/ca.pem"},
		)
	}

	if orchard.Bitcoin != nil {
		envVars = append(envVars,
			corev1.EnvVar{Name: "BITCOIN_TYPE", Value: orchard.Bitcoin.Type},
			corev1.EnvVar{Name: "BITCOIN_RPC_HOST", Value: orchard.Bitcoin.RPCHost},
			corev1.EnvVar{Name: "BITCOIN_RPC_PORT", Value: strconv.Itoa(int(orchard.Bitcoin.RPCPort))},
			corev1.EnvVar{
				Name: "BITCOIN_RPC_USER",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: orchard.Bitcoin.RPCUserSecretRef,
				},
			},
			corev1.EnvVar{
				Name: "BITCOIN_RPC_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: orchard.Bitcoin.RPCPasswordSecretRef,
				},
			},
		)
	}

	if orchard.Lightning != nil {
		envVars = append(envVars,
			corev1.EnvVar{Name: "LIGHTNING_TYPE", Value: orchard.Lightning.Type},
			corev1.EnvVar{Name: "LIGHTNING_RPC_HOST", Value: orchard.Lightning.RPCHost},
			corev1.EnvVar{Name: "LIGHTNING_RPC_PORT", Value: strconv.Itoa(int(orchard.Lightning.RPCPort))},
		)
		if orchard.Lightning.MacaroonSecretRef != nil {
			envVars = append(envVars, corev1.EnvVar{Name: "LIGHTNING_MACAROON", Value: orchardLightningMacaroonPath})
		}
		if orchard.Lightning.CertSecretRef != nil {
			envVars = append(envVars, corev1.EnvVar{Name: "LIGHTNING_CERT", Value: orchardLightningCertPath})
		}
		if orchard.Lightning.KeySecretRef != nil {
			envVars = append(envVars, corev1.EnvVar{Name: "LIGHTNING_KEY", Value: orchardLightningKeyPath})
		}
		if orchard.Lightning.CASecretRef != nil {
			envVars = append(envVars, corev1.EnvVar{Name: "LIGHTNING_CA", Value: orchardLightningCAPath})
		}
	}

	if orchard.TaprootAssets != nil {
		envVars = append(envVars,
			corev1.EnvVar{Name: "TAPROOT_ASSETS_TYPE", Value: orchard.TaprootAssets.Type},
			corev1.EnvVar{Name: "TAPROOT_ASSETS_RPC_HOST", Value: orchard.TaprootAssets.RPCHost},
			corev1.EnvVar{Name: "TAPROOT_ASSETS_RPC_PORT", Value: strconv.Itoa(int(orchard.TaprootAssets.RPCPort))},
		)
		if orchard.TaprootAssets.MacaroonSecretRef != nil {
			envVars = append(envVars, corev1.EnvVar{Name: "TAPROOT_ASSETS_MACAROON", Value: orchardTaprootMacaroonPath})
		}
		if orchard.TaprootAssets.CertSecretRef != nil {
			envVars = append(envVars, corev1.EnvVar{Name: "TAPROOT_ASSETS_CERT", Value: orchardTaprootCertPath})
		}
	}

	if orchard.AI != nil {
		envVars = append(envVars, corev1.EnvVar{Name: "AI_API", Value: orchard.AI.API})
	}

	envVars = append(envVars, orchard.ExtraEnv...)

	return envVars
}

func generateOrchardMintDatabaseEnvVar(mint *mintv1alpha1.CashuMint) *corev1.EnvVar {
	orchard := mint.Spec.Orchard
	if orchard != nil && orchard.Mint != nil && orchard.Mint.Database != "" {
		return &corev1.EnvVar{Name: "MINT_DATABASE", Value: orchard.Mint.Database}
	}

	switch mint.Spec.Database.Engine {
	case mintv1alpha1.DatabaseEngineSQLite:
		return &corev1.EnvVar{Name: "MINT_DATABASE", Value: orchardMintSQLitePath}
	case mintv1alpha1.DatabaseEnginePostgres:
		if mint.Spec.Database.Postgres == nil {
			return nil
		}
		if mint.Spec.Database.Postgres.AutoProvision {
			return &corev1.EnvVar{
				Name: "MINT_DATABASE",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: mint.Name + "-postgres-secret"},
						Key:                  "database-url",
					},
				},
			}
		}
		if mint.Spec.Database.Postgres.URLSecretRef != nil {
			return &corev1.EnvVar{
				Name: "MINT_DATABASE",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: mint.Spec.Database.Postgres.URLSecretRef,
				},
			}
		}
		if mint.Spec.Database.Postgres.URL != "" {
			return &corev1.EnvVar{Name: "MINT_DATABASE", Value: mint.Spec.Database.Postgres.URL}
		}
	}

	return nil
}

func generateOrchardVolumeMounts(mint *mintv1alpha1.CashuMint) []corev1.VolumeMount {
	mounts := []corev1.VolumeMount{
		{
			Name:      orchardDataVolumeName,
			MountPath: orchardDataDir,
		},
		{
			Name:      orchardTmpVolumeName,
			MountPath: orchardTmpDir,
		},
		{
			Name:      "data",
			MountPath: orchardMintDataDir,
		},
	}

	if orchardMintRPCMTLS(mint) {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      managementRPCTLSVolumeName,
			MountPath: orchardManagementRPCTLSMountPath,
			ReadOnly:  true,
		})
	}

	if orchard := mint.Spec.Orchard; orchard != nil {
		if orchard.Lightning != nil {
			if orchard.Lightning.MacaroonSecretRef != nil {
				mounts = append(mounts, corev1.VolumeMount{
					Name:      orchardLightningMacaroonVolumeName,
					MountPath: orchardLightningMacaroonPath,
					SubPath:   orchard.Lightning.MacaroonSecretRef.Key,
					ReadOnly:  true,
				})
			}
			if orchard.Lightning.CertSecretRef != nil {
				mounts = append(mounts, corev1.VolumeMount{
					Name:      orchardLightningCertVolumeName,
					MountPath: orchardLightningCertPath,
					SubPath:   orchard.Lightning.CertSecretRef.Key,
					ReadOnly:  true,
				})
			}
			if orchard.Lightning.KeySecretRef != nil {
				mounts = append(mounts, corev1.VolumeMount{
					Name:      orchardLightningKeyVolumeName,
					MountPath: orchardLightningKeyPath,
					SubPath:   orchard.Lightning.KeySecretRef.Key,
					ReadOnly:  true,
				})
			}
			if orchard.Lightning.CASecretRef != nil {
				mounts = append(mounts, corev1.VolumeMount{
					Name:      orchardLightningCAVolumeName,
					MountPath: orchardLightningCAPath,
					SubPath:   orchard.Lightning.CASecretRef.Key,
					ReadOnly:  true,
				})
			}
		}

		if orchard.TaprootAssets != nil {
			if orchard.TaprootAssets.MacaroonSecretRef != nil {
				mounts = append(mounts, corev1.VolumeMount{
					Name:      orchardTaprootMacaroonVolumeName,
					MountPath: orchardTaprootMacaroonPath,
					SubPath:   orchard.TaprootAssets.MacaroonSecretRef.Key,
					ReadOnly:  true,
				})
			}
			if orchard.TaprootAssets.CertSecretRef != nil {
				mounts = append(mounts, corev1.VolumeMount{
					Name:      orchardTaprootCertVolumeName,
					MountPath: orchardTaprootCertPath,
					SubPath:   orchard.TaprootAssets.CertSecretRef.Key,
					ReadOnly:  true,
				})
			}
		}
	}

	return mounts
}

// GenerateOrchardVolumes creates additional Orchard-specific pod volumes.
func GenerateOrchardVolumes(mint *mintv1alpha1.CashuMint) []corev1.Volume {
	if !orchardEnabled(mint) {
		return nil
	}

	volumes := []corev1.Volume{
		{
			Name: orchardDataVolumeName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: orchardPVCName(mint),
				},
			},
		},
		{
			Name: orchardTmpVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}

	orchard := mint.Spec.Orchard
	if orchard.Lightning != nil {
		if orchard.Lightning.MacaroonSecretRef != nil {
			volumes = append(volumes, corev1.Volume{
				Name: orchardLightningMacaroonVolumeName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{SecretName: orchard.Lightning.MacaroonSecretRef.Name},
				},
			})
		}
		if orchard.Lightning.CertSecretRef != nil {
			volumes = append(volumes, corev1.Volume{
				Name: orchardLightningCertVolumeName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{SecretName: orchard.Lightning.CertSecretRef.Name},
				},
			})
		}
		if orchard.Lightning.KeySecretRef != nil {
			volumes = append(volumes, corev1.Volume{
				Name: orchardLightningKeyVolumeName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{SecretName: orchard.Lightning.KeySecretRef.Name},
				},
			})
		}
		if orchard.Lightning.CASecretRef != nil {
			volumes = append(volumes, corev1.Volume{
				Name: orchardLightningCAVolumeName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{SecretName: orchard.Lightning.CASecretRef.Name},
				},
			})
		}
	}

	if orchard.TaprootAssets != nil {
		if orchard.TaprootAssets.MacaroonSecretRef != nil {
			volumes = append(volumes, corev1.Volume{
				Name: orchardTaprootMacaroonVolumeName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{SecretName: orchard.TaprootAssets.MacaroonSecretRef.Name},
				},
			})
		}
		if orchard.TaprootAssets.CertSecretRef != nil {
			volumes = append(volumes, corev1.Volume{
				Name: orchardTaprootCertVolumeName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{SecretName: orchard.TaprootAssets.CertSecretRef.Name},
				},
			})
		}
	}

	return volumes
}

// GenerateOrchardPVC creates a PersistentVolumeClaim for Orchard application data.
func GenerateOrchardPVC(mint *mintv1alpha1.CashuMint, scheme *runtime.Scheme) (*corev1.PersistentVolumeClaim, error) {
	if !orchardEnabled(mint) {
		return nil, nil
	}

	size := mintv1alpha1.DefaultStorageSize
	if mint.Spec.Orchard.Storage != nil && mint.Spec.Orchard.Storage.Size != "" {
		size = mint.Spec.Orchard.Storage.Size
	}

	pvc := &corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "PersistentVolumeClaim",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      orchardPVCName(mint),
			Namespace: mint.Namespace,
			Labels:    orchardLabels(mint),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(size),
				},
			},
		},
	}

	if mint.Spec.Orchard.Storage != nil && mint.Spec.Orchard.Storage.StorageClassName != nil {
		pvc.Spec.StorageClassName = mint.Spec.Orchard.Storage.StorageClassName
	}

	if err := controllerutil.SetControllerReference(mint, pvc, scheme); err != nil {
		return nil, fmt.Errorf("failed to set controller reference: %w", err)
	}

	return pvc, nil
}

// GenerateOrchardService creates a Service for Orchard.
func GenerateOrchardService(mint *mintv1alpha1.CashuMint, scheme *runtime.Scheme) (*corev1.Service, error) {
	if !orchardEnabled(mint) {
		return nil, nil
	}

	serviceType := corev1.ServiceTypeClusterIP
	annotations := map[string]string{}
	var loadBalancerIP string
	if mint.Spec.Orchard.Service != nil {
		if mint.Spec.Orchard.Service.Type != "" {
			serviceType = mint.Spec.Orchard.Service.Type
		}
		if mint.Spec.Orchard.Service.Annotations != nil {
			annotations = mint.Spec.Orchard.Service.Annotations
		}
		loadBalancerIP = mint.Spec.Orchard.Service.LoadBalancerIP
	}

	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        orchardResourceName(mint),
			Namespace:   mint.Namespace,
			Labels:      orchardLabels(mint),
			Annotations: annotations,
		},
		Spec: corev1.ServiceSpec{
			Type:     serviceType,
			Selector: mintSelectorLabels(mint),
			Ports: []corev1.ServicePort{
				{
					Name:       orchardStr,
					Port:       orchardPort(mint),
					TargetPort: intstr.FromString(orchardStr),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}

	if serviceType == corev1.ServiceTypeLoadBalancer && loadBalancerIP != "" {
		service.Spec.LoadBalancerIP = loadBalancerIP
	}

	if err := controllerutil.SetControllerReference(mint, service, scheme); err != nil {
		return nil, fmt.Errorf("failed to set controller reference: %w", err)
	}

	return service, nil
}

// GenerateOrchardIngress creates an Ingress for Orchard.
func GenerateOrchardIngress(mint *mintv1alpha1.CashuMint, scheme *runtime.Scheme) (*networkingv1.Ingress, error) {
	if !orchardEnabled(mint) || mint.Spec.Orchard.Ingress == nil || !mint.Spec.Orchard.Ingress.Enabled {
		return nil, nil
	}

	pathTypePrefix := networkingv1.PathTypePrefix
	ingressClassName := mint.Spec.Orchard.Ingress.ClassName
	if ingressClassName == "" {
		ingressClassName = mintv1alpha1.DefaultIngressClassName
	}

	annotations := map[string]string{
		"nginx.ingress.kubernetes.io/ssl-redirect":      trueStr,
		"nginx.ingress.kubernetes.io/backend-protocol":  "HTTP",
		"nginx.ingress.kubernetes.io/enable-cors":       trueStr,
		"nginx.ingress.kubernetes.io/cors-allow-origin": "*",
	}
	if mint.Spec.Orchard.Ingress.TLS != nil && mint.Spec.Orchard.Ingress.TLS.Enabled &&
		mint.Spec.Orchard.Ingress.TLS.CertManager != nil && mint.Spec.Orchard.Ingress.TLS.CertManager.Enabled {
		issuerKind := mint.Spec.Orchard.Ingress.TLS.CertManager.IssuerKind
		if issuerKind == "" {
			issuerKind = mintv1alpha1.DefaultClusterIssuerKind
		}
		annotations["cert-manager.io/issuer"] = mint.Spec.Orchard.Ingress.TLS.CertManager.IssuerName
		annotations["cert-manager.io/issuer-kind"] = issuerKind
	}
	if mint.Spec.Orchard.Ingress.Annotations != nil {
		for k, v := range mint.Spec.Orchard.Ingress.Annotations {
			annotations[k] = v
		}
	}

	ingress := &networkingv1.Ingress{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "networking.k8s.io/v1",
			Kind:       "Ingress",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        orchardResourceName(mint),
			Namespace:   mint.Namespace,
			Labels:      orchardLabels(mint),
			Annotations: annotations,
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: &ingressClassName,
			Rules: []networkingv1.IngressRule{
				{
					Host: mint.Spec.Orchard.Ingress.Host,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: &pathTypePrefix,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: orchardResourceName(mint),
											Port: networkingv1.ServiceBackendPort{
												Number: orchardPort(mint),
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	if mint.Spec.Orchard.Ingress.TLS != nil && mint.Spec.Orchard.Ingress.TLS.Enabled {
		secretName := mint.Spec.Orchard.Ingress.TLS.SecretName
		if secretName == "" {
			secretName = orchardResourceName(mint) + "-tls"
		}
		ingress.Spec.TLS = []networkingv1.IngressTLS{
			{
				Hosts:      []string{mint.Spec.Orchard.Ingress.Host},
				SecretName: secretName,
			},
		}
	}

	if err := controllerutil.SetControllerReference(mint, ingress, scheme); err != nil {
		return nil, fmt.Errorf("failed to set controller reference: %w", err)
	}

	return ingress, nil
}

func orchardPort(mint *mintv1alpha1.CashuMint) int32 {
	if mint.Spec.Orchard != nil && mint.Spec.Orchard.Port != 0 {
		return mint.Spec.Orchard.Port
	}
	return orchardPortDefault
}

func orchardMintType(mint *mintv1alpha1.CashuMint) string {
	if mint.Spec.Orchard != nil && mint.Spec.Orchard.Mint != nil && mint.Spec.Orchard.Mint.Type != "" {
		return mint.Spec.Orchard.Mint.Type
	}
	return "cdk"
}

func orchardMintAPI(mint *mintv1alpha1.CashuMint) string {
	if mint.Spec.Orchard != nil && mint.Spec.Orchard.Mint != nil && mint.Spec.Orchard.Mint.API != "" {
		return mint.Spec.Orchard.Mint.API
	}
	port := mint.Spec.MintInfo.ListenPort
	if port == 0 {
		port = 8085
	}
	return fmt.Sprintf("http://%s:%d", mintv1alpha1.DefaultLoopbackHost, port)
}

func orchardMintRPCHost(mint *mintv1alpha1.CashuMint) string {
	if mint.Spec.Orchard != nil && mint.Spec.Orchard.Mint != nil && mint.Spec.Orchard.Mint.RPC != nil && mint.Spec.Orchard.Mint.RPC.Host != "" {
		return mint.Spec.Orchard.Mint.RPC.Host
	}
	return mintv1alpha1.DefaultLoopbackHost
}

func orchardMintRPCPort(mint *mintv1alpha1.CashuMint) int32 {
	if mint.Spec.Orchard != nil && mint.Spec.Orchard.Mint != nil && mint.Spec.Orchard.Mint.RPC != nil && mint.Spec.Orchard.Mint.RPC.Port != 0 {
		return mint.Spec.Orchard.Mint.RPC.Port
	}
	if mint.Spec.ManagementRPC != nil && mint.Spec.ManagementRPC.Port != 0 {
		return mint.Spec.ManagementRPC.Port
	}
	return 8086
}

func orchardMintRPCMTLS(mint *mintv1alpha1.CashuMint) bool {
	if mint.Spec.Orchard != nil && mint.Spec.Orchard.Mint != nil && mint.Spec.Orchard.Mint.RPC != nil && mint.Spec.Orchard.Mint.RPC.MTLS != nil {
		return *mint.Spec.Orchard.Mint.RPC.MTLS
	}
	return mintv1alpha1.ManagementRPCTLSEnabled(&mint.Spec)
}

func getOrchardResourceRequirements(mint *mintv1alpha1.CashuMint) corev1.ResourceRequirements {
	if mint.Spec.Orchard != nil && mint.Spec.Orchard.Resources != nil {
		return *mint.Spec.Orchard.Resources
	}

	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("100m"),
			corev1.ResourceMemory: resource.MustParse("128Mi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("500m"),
			corev1.ResourceMemory: resource.MustParse("512Mi"),
		},
	}
}

func getOrchardContainerSecurityContext(mint *mintv1alpha1.CashuMint) *corev1.SecurityContext {
	if mint.Spec.Orchard != nil && mint.Spec.Orchard.ContainerSecurityContext != nil {
		return mint.Spec.Orchard.ContainerSecurityContext
	}
	return &corev1.SecurityContext{
		AllowPrivilegeEscalation: boolPtr(false),
		ReadOnlyRootFilesystem:   boolPtr(false),
		RunAsNonRoot:             boolPtr(false),
		RunAsUser:                int64Ptr(0),
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		},
	}
}

func orchardSpecifiesMintDatabaseTLS(o *mintv1alpha1.OrchardConfig) bool {
	return o != nil &&
		o.Mint != nil &&
		(o.Mint.DatabaseCASecretRef != nil || o.Mint.DatabaseCertSecretRef != nil || o.Mint.DatabaseKeySecretRef != nil)
}
