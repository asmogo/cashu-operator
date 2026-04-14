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
	"crypto/rand"
	"encoding/base64"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	mintv1alpha1 "github.com/asmogo/cashu-operator/api/v1alpha1"
)

// initPasswordScript is the shell script for the init-password container.
// It ensures the PostgreSQL password matches the Secret when the database
// has already been initialized (e.g. after CR delete/recreate where the PVC
// survives but the Secret was regenerated).
const initPasswordScript = `set -e
if [ -f /var/lib/postgresql/data/pgdata/PG_VERSION ]; then
  echo "Database already initialized, ensuring password is correct..."
  chown -R 999:999 /var/lib/postgresql/data/pgdata
  chmod 0700 /var/lib/postgresql/data/pgdata
  su-exec 999:999 pg_ctl -D /var/lib/postgresql/data/pgdata -o "-c listen_addresses='' -c local_preload_libraries=''" -w start
  su-exec 999:999 psql -U ` + postgresUser + ` -d ` + postgresDatabase + ` -c "ALTER USER ` + postgresUser + ` WITH PASSWORD '${POSTGRES_PASSWORD}';"
  su-exec 999:999 pg_ctl -D /var/lib/postgresql/data/pgdata -w stop
  echo "Password updated successfully"
else
  echo "Database not yet initialized, skipping password update"
fi`

const (
	postgresUser     = "cdk"
	postgresDatabase = "cdk_mintd"
)

// GeneratePostgresSecret creates a Secret for PostgreSQL credentials.
// The Secret is intentionally created WITHOUT an owner reference so that it
// survives CR deletion (matching the StatefulSet PVC lifecycle). This prevents
// password mismatch when a CashuMint is deleted and recreated while the PVC
// still contains data initialized with the original password.
// existingPassword should be provided if the secret already exists to avoid regenerating the password.
func GeneratePostgresSecret(mint *mintv1alpha1.CashuMint, existingPassword string) (*corev1.Secret, error) {
	if mint.Spec.Database.Postgres == nil || !mint.Spec.Database.Postgres.AutoProvision {
		return nil, nil
	}

	labels := map[string]string{
		"app.kubernetes.io/name":       "postgresql",
		"app.kubernetes.io/instance":   mint.Name,
		"app.kubernetes.io/component":  "database",
		"app.kubernetes.io/managed-by": "cashu-operator",
	}

	// Use existing password or generate a new one
	password := existingPassword
	if password == "" {
		var err error
		password, err = generateSecurePassword(32)
		if err != nil {
			return nil, fmt.Errorf("failed to generate password: %w", err)
		}
	}

	// Construct database URL
	// Use sslmode=disable for auto-provisioned postgres (internal cluster communication)
	dbURL := fmt.Sprintf("postgresql://%s:%s@%s-postgres:5432/%s?sslmode=disable",
		postgresUser, password, mint.Name, postgresDatabase)

	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      mint.Name + "-postgres-secret",
			Namespace: mint.Namespace,
			Labels:    labels,
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			"password":     password,
			"database-url": dbURL,
		},
	}

	return secret, nil
}

// GeneratePostgresService creates a Service for PostgreSQL
func GeneratePostgresService(mint *mintv1alpha1.CashuMint, scheme *runtime.Scheme) (*corev1.Service, error) {
	if mint.Spec.Database.Postgres == nil || !mint.Spec.Database.Postgres.AutoProvision {
		return nil, nil
	}

	labels := map[string]string{
		"app.kubernetes.io/name":       "postgresql",
		"app.kubernetes.io/instance":   mint.Name,
		"app.kubernetes.io/component":  "database",
		"app.kubernetes.io/managed-by": "cashu-operator",
	}

	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      mint.Name + "-postgres",
			Namespace: mint.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: labels,
			Ports: []corev1.ServicePort{
				{
					Name:     "postgres",
					Port:     5432,
					Protocol: corev1.ProtocolTCP,
				},
			},
			ClusterIP: "None", // Headless service for StatefulSet
		},
	}

	if err := controllerutil.SetControllerReference(mint, service, scheme); err != nil {
		return nil, fmt.Errorf("failed to set controller reference: %w", err)
	}

	return service, nil
}

// GeneratePostgresStatefulSet creates a StatefulSet for PostgreSQL
func GeneratePostgresStatefulSet(mint *mintv1alpha1.CashuMint, scheme *runtime.Scheme) (*appsv1.StatefulSet, error) {
	if mint.Spec.Database.Postgres == nil || !mint.Spec.Database.Postgres.AutoProvision {
		return nil, nil
	}

	spec := mint.Spec.Database.Postgres.AutoProvisionSpec
	if spec == nil {
		spec = &mintv1alpha1.PostgresAutoProvisionSpec{
			StorageSize: mintv1alpha1.DefaultStorageSize,
			Version:     "15",
		}
	}

	labels := map[string]string{
		"app.kubernetes.io/name":       "postgresql",
		"app.kubernetes.io/instance":   mint.Name,
		"app.kubernetes.io/component":  "database",
		"app.kubernetes.io/managed-by": "cashu-operator",
	}

	replicas := int32(1)
	version := spec.Version
	if version == "" {
		version = "15"
	}

	storageSize := spec.StorageSize
	if storageSize == "" {
		storageSize = mintv1alpha1.DefaultStorageSize
	}

	statefulSet := &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "StatefulSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      mint.Name + "-postgres",
			Namespace: mint.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    &replicas,
			ServiceName: mint.Name + "-postgres",
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{
						{
							Name:  "init-password",
							Image: fmt.Sprintf("postgres:%s-alpine", version),
							Command: []string{
								"sh",
								"-c",
								initPasswordScript,
							},
							Env: []corev1.EnvVar{
								{
									Name: "POSTGRES_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: mint.Name + "-postgres-secret",
											},
											Key: "password",
										},
									},
								},
								{
									Name:  "PGDATA",
									Value: "/var/lib/postgresql/data/pgdata",
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "data",
									MountPath: "/var/lib/postgresql/data",
								},
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: boolPtr(false),
								RunAsNonRoot:             boolPtr(false), // Need root to run su-exec
								RunAsUser:                int64Ptr(0),
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:  "postgres",
							Image: fmt.Sprintf("postgres:%s-alpine", version),
							Env: []corev1.EnvVar{
								{
									Name:  "POSTGRES_DB",
									Value: postgresDatabase,
								},
								{
									Name:  "POSTGRES_USER",
									Value: postgresUser,
								},
								{
									Name: "POSTGRES_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: mint.Name + "-postgres-secret",
											},
											Key: "password",
										},
									},
								},
								{
									Name:  "PGDATA",
									Value: "/var/lib/postgresql/data/pgdata",
								},
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "postgres",
									ContainerPort: 5432,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "data",
									MountPath: "/var/lib/postgresql/data",
								},
							},
							Resources: getPostgresResources(spec),
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									Exec: &corev1.ExecAction{
										Command: []string{
											"pg_isready",
											"-U", postgresUser,
											"-d", postgresDatabase,
										},
									},
								},
								InitialDelaySeconds: 30,
								PeriodSeconds:       10,
								TimeoutSeconds:      5,
								FailureThreshold:    3,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									Exec: &corev1.ExecAction{
										Command: []string{
											"pg_isready",
											"-U", postgresUser,
											"-d", postgresDatabase,
										},
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       5,
								TimeoutSeconds:      3,
								FailureThreshold:    3,
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: boolPtr(false),
								RunAsNonRoot:             boolPtr(true),
								RunAsUser:                int64Ptr(999), // postgres user in alpine image
							},
						},
					},
					SecurityContext: &corev1.PodSecurityContext{
						FSGroup: int64Ptr(999),
					},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "data",
						Labels: labels,
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{
							corev1.ReadWriteOnce,
						},
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse(storageSize),
							},
						},
						StorageClassName: spec.StorageClassName,
					},
				},
			},
		},
	}

	if err := controllerutil.SetControllerReference(mint, statefulSet, scheme); err != nil {
		return nil, fmt.Errorf("failed to set controller reference: %w", err)
	}

	return statefulSet, nil
}

// getPostgresResources returns resource requirements for PostgreSQL
func getPostgresResources(spec *mintv1alpha1.PostgresAutoProvisionSpec) corev1.ResourceRequirements {
	if spec != nil && spec.Resources != nil {
		return *spec.Resources
	}

	// Default resource requirements
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("100m"),
			corev1.ResourceMemory: resource.MustParse("256Mi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1000m"),
			corev1.ResourceMemory: resource.MustParse("1Gi"),
		},
	}
}

// generateSecurePassword generates a cryptographically secure random password
func generateSecurePassword(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes)[:length], nil
}
