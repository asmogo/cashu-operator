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
	"k8s.io/apimachinery/pkg/util/intstr"

	corev1 "k8s.io/api/core/v1"

	mintv1alpha1 "github.com/asmogo/cashu-operator/api/v1alpha1"
)

// ProbeBuilder provides a fluent interface for constructing Kubernetes Probes.
// This reduces boilerplate and improves readability in resource generation code.
type ProbeBuilder struct {
	probe *corev1.Probe
}

// NewProbeBuilder creates a new probe builder with sensible defaults.
func NewProbeBuilder() *ProbeBuilder {
	return &ProbeBuilder{
		probe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Scheme: corev1.URISchemeHTTP,
				},
			},
		},
	}
}

// WithPath sets the HTTP path for the probe.
func (pb *ProbeBuilder) WithPath(path string) *ProbeBuilder {
	pb.probe.ProbeHandler.HTTPGet.Path = path
	return pb
}

// WithPort sets the port for the probe.
func (pb *ProbeBuilder) WithPort(port string) *ProbeBuilder {
	pb.probe.ProbeHandler.HTTPGet.Port = intstr.FromString(port)
	return pb
}

// WithInitialDelay sets the initial delay in seconds.
func (pb *ProbeBuilder) WithInitialDelay(seconds int32) *ProbeBuilder {
	pb.probe.InitialDelaySeconds = seconds
	return pb
}

// WithPeriod sets the period in seconds.
func (pb *ProbeBuilder) WithPeriod(seconds int32) *ProbeBuilder {
	pb.probe.PeriodSeconds = seconds
	return pb
}

// WithTimeout sets the timeout in seconds.
func (pb *ProbeBuilder) WithTimeout(seconds int32) *ProbeBuilder {
	pb.probe.TimeoutSeconds = seconds
	return pb
}

// WithFailureThreshold sets the failure threshold.
func (pb *ProbeBuilder) WithFailureThreshold(threshold int32) *ProbeBuilder {
	pb.probe.FailureThreshold = threshold
	return pb
}

// Build returns the constructed Probe.
func (pb *ProbeBuilder) Build() *corev1.Probe {
	return pb.probe
}

// ContainerBuilder provides a fluent interface for building Kubernetes Containers.
type ContainerBuilder struct {
	container *corev1.Container
}

// NewContainerBuilder creates a new container builder.
func NewContainerBuilder(name string) *ContainerBuilder {
	return &ContainerBuilder{
		container: &corev1.Container{
			Name:      name,
			Resources: corev1.ResourceRequirements{},
		},
	}
}

// WithImage sets the container image.
func (cb *ContainerBuilder) WithImage(image string) *ContainerBuilder {
	cb.container.Image = image
	return cb
}

// WithImagePullPolicy sets the image pull policy.
func (cb *ContainerBuilder) WithImagePullPolicy(policy corev1.PullPolicy) *ContainerBuilder {
	cb.container.ImagePullPolicy = policy
	return cb
}

// WithPort adds a container port.
func (cb *ContainerBuilder) WithPort(name string, port int32) *ContainerBuilder {
	cb.container.Ports = append(cb.container.Ports, corev1.ContainerPort{
		Name:          name,
		ContainerPort: port,
		Protocol:      corev1.ProtocolTCP,
	})
	return cb
}

// WithEnv adds an environment variable.
func (cb *ContainerBuilder) WithEnv(name, value string) *ContainerBuilder {
	cb.container.Env = append(cb.container.Env, corev1.EnvVar{
		Name:  name,
		Value: value,
	})
	return cb
}

// WithEnvFrom adds a complete EnvVar.
func (cb *ContainerBuilder) WithEnvFrom(env corev1.EnvVar) *ContainerBuilder {
	cb.container.Env = append(cb.container.Env, env)
	return cb
}

// WithVolumeMount adds a volume mount.
func (cb *ContainerBuilder) WithVolumeMount(name, path string, readOnly bool) *ContainerBuilder {
	cb.container.VolumeMounts = append(cb.container.VolumeMounts, corev1.VolumeMount{
		Name:      name,
		MountPath: path,
		ReadOnly:  readOnly,
	})
	return cb
}

// WithLivenessProbe sets the liveness probe.
func (cb *ContainerBuilder) WithLivenessProbe(probe *corev1.Probe) *ContainerBuilder {
	cb.container.LivenessProbe = probe
	return cb
}

// WithReadinessProbe sets the readiness probe.
func (cb *ContainerBuilder) WithReadinessProbe(probe *corev1.Probe) *ContainerBuilder {
	cb.container.ReadinessProbe = probe
	return cb
}

// WithResources sets resource requirements.
func (cb *ContainerBuilder) WithResources(resources corev1.ResourceRequirements) *ContainerBuilder {
	cb.container.Resources = resources
	return cb
}

// WithSecurityContext sets the security context.
func (cb *ContainerBuilder) WithSecurityContext(sc *corev1.SecurityContext) *ContainerBuilder {
	cb.container.SecurityContext = sc
	return cb
}

// Build returns the constructed Container.
func (cb *ContainerBuilder) Build() corev1.Container {
	return *cb.container
}

// EnvVarBuilder provides utilities for building environment variables from various sources.
type EnvVarBuilder struct{}

// NewEnvVarBuilder creates a new environment variable builder.
func NewEnvVarBuilder() *EnvVarBuilder {
	return &EnvVarBuilder{}
}

// FromConfigMapKey creates an EnvVar from a ConfigMap key.
func (evb *EnvVarBuilder) FromConfigMapKey(name, configMapName, key string) corev1.EnvVar {
	return corev1.EnvVar{
		Name: name,
		ValueFrom: &corev1.EnvVarSource{
			ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: configMapName,
				},
				Key: key,
			},
		},
	}
}

// FromSecretKey creates an EnvVar from a Secret key.
func (evb *EnvVarBuilder) FromSecretKey(name, secretName, key string) corev1.EnvVar {
	return corev1.EnvVar{
		Name: name,
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: secretName,
				},
				Key: key,
			},
		},
	}
}

// FromSecretKeyRef creates an EnvVar from a SecretKeySelector.
func (evb *EnvVarBuilder) FromSecretKeyRef(name string, ref *corev1.SecretKeySelector) corev1.EnvVar {
	return corev1.EnvVar{
		Name: name,
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: ref,
		},
	}
}

// LabelSelector provides utilities for common label selectors.
type LabelSelector struct{}

// NewLabelSelector creates a new label selector helper.
func NewLabelSelector() *LabelSelector {
	return &LabelSelector{}
}

// ForMint returns standard labels for a mint resource.
func (ls *LabelSelector) ForMint(mint *mintv1alpha1.CashuMint) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "cashu-mint",
		"app.kubernetes.io/instance":   mint.Name,
		"app.kubernetes.io/managed-by": "cashu-operator",
	}
}

// WithComponent adds a component label.
func (ls *LabelSelector) WithComponent(labels map[string]string, component string) map[string]string {
	labels["app.kubernetes.io/component"] = component
	return labels
}

// VolumeBuilder provides utilities for building Kubernetes volumes.
type VolumeBuilder struct{}

// NewVolumeBuilder creates a new volume builder.
func NewVolumeBuilder() *VolumeBuilder {
	return &VolumeBuilder{}
}

// ConfigMapVolume creates a volume from a ConfigMap.
func (vb *VolumeBuilder) ConfigMapVolume(name, configMapName string) corev1.Volume {
	return corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: configMapName,
				},
			},
		},
	}
}

// SecretVolume creates a volume from a Secret.
func (vb *VolumeBuilder) SecretVolume(name, secretName string) corev1.Volume {
	return corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: secretName,
			},
		},
	}
}

// PVCVolume creates a volume from a PersistentVolumeClaim.
func (vb *VolumeBuilder) PVCVolume(name, claimName string) corev1.Volume {
	return corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: claimName,
			},
		},
	}
}

// EmptyDirVolume creates an ephemeral emptyDir volume.
func (vb *VolumeBuilder) EmptyDirVolume(name string) corev1.Volume {
	return corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
}

// Helper functions for pointer types used throughout generators

// BoolPtr creates a pointer to a bool value.
func BoolPtr(b bool) *bool {
	return &b
}

// Int32Ptr creates a pointer to an int32 value.
func Int32Ptr(i int32) *int32 {
	return &i
}

// Int64Ptr creates a pointer to an int64 value.
func Int64Ptr(i int64) *int64 {
	return &i
}

// StringPtr creates a pointer to a string value.
func StringPtr(s string) *string {
	return &s
}
