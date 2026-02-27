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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	mintv1alpha1 "github.com/asmogo/cashu-operator/api/v1alpha1"
)

const (
	RestoreObjectKeyAnnotation = "mint.cashu.asmogo.github.io/restore-object-key"
	RestoreRequestIDAnnotation = "mint.cashu.asmogo.github.io/restore-request-id"
)

// GeneratePostgresBackupCronJob creates a CronJob for scheduled PostgreSQL backups to S3-compatible storage.
func GeneratePostgresBackupCronJob(mint *mintv1alpha1.CashuMint, scheme *runtime.Scheme) (*batchv1.CronJob, error) {
	if mint.Spec.Backup == nil || !mint.Spec.Backup.Enabled {
		return nil, nil
	}
	if mint.Spec.Database.Engine != "postgres" ||
		mint.Spec.Database.Postgres == nil ||
		!mint.Spec.Database.Postgres.AutoProvision ||
		mint.Spec.Backup.S3 == nil {
		return nil, nil
	}

	labels := map[string]string{
		"app.kubernetes.io/name":       "cashu-mint",
		"app.kubernetes.io/instance":   mint.Name,
		"app.kubernetes.io/component":  "backup",
		"app.kubernetes.io/managed-by": "cashu-operator",
	}

	schedule := mint.Spec.Backup.Schedule
	if schedule == "" {
		schedule = "0 */6 * * *"
	}

	history := int32(14)
	if mint.Spec.Backup.RetentionCount != nil {
		history = *mint.Spec.Backup.RetentionCount
	}

	pgVersion := "15"
	if mint.Spec.Database.Postgres.AutoProvisionSpec != nil &&
		mint.Spec.Database.Postgres.AutoProvisionSpec.Version != "" {
		pgVersion = mint.Spec.Database.Postgres.AutoProvisionSpec.Version
	}

	env := []corev1.EnvVar{
		{
			Name: "DATABASE_URL",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: mint.Name + "-postgres-secret",
					},
					Key: "database-url",
				},
			},
		},
		{
			Name:  "MINT_NAME",
			Value: mint.Name,
		},
		{
			Name:  "S3_BUCKET",
			Value: mint.Spec.Backup.S3.Bucket,
		},
		{
			Name:  "S3_PREFIX",
			Value: mint.Spec.Backup.S3.Prefix,
		},
		{
			Name: "AWS_ACCESS_KEY_ID",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &mint.Spec.Backup.S3.AccessKeyIDSecretRef,
			},
		},
		{
			Name: "AWS_SECRET_ACCESS_KEY",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &mint.Spec.Backup.S3.SecretAccessKeySecretRef,
			},
		},
	}

	if mint.Spec.Backup.S3.Region != "" {
		env = append(env, corev1.EnvVar{
			Name:  "AWS_DEFAULT_REGION",
			Value: mint.Spec.Backup.S3.Region,
		})
	}
	if mint.Spec.Backup.S3.Endpoint != "" {
		env = append(env, corev1.EnvVar{
			Name:  "S3_ENDPOINT",
			Value: mint.Spec.Backup.S3.Endpoint,
		})
	}

	cronJob := &batchv1.CronJob{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "batch/v1",
			Kind:       "CronJob",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      mint.Name + "-backup",
			Namespace: mint.Namespace,
			Labels:    labels,
		},
		Spec: batchv1.CronJobSpec{
			Schedule:                   schedule,
			ConcurrencyPolicy:          batchv1.ForbidConcurrent,
			SuccessfulJobsHistoryLimit: &history,
			FailedJobsHistoryLimit:     &history,
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: labels,
						},
						Spec: corev1.PodSpec{
							RestartPolicy: corev1.RestartPolicyOnFailure,
							Containers: []corev1.Container{
								{
									Name:  "postgres-backup",
									Image: fmt.Sprintf("postgres:%s-alpine", pgVersion),
									Command: []string{
										"sh",
										"-ec",
										`set -euo pipefail
apk add --no-cache aws-cli >/dev/null
timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
tmp_file="/tmp/${MINT_NAME}-${timestamp}.sql.gz"
object_key="${MINT_NAME}-${timestamp}.sql.gz"
if [ -n "${S3_PREFIX:-}" ]; then
  prefix="${S3_PREFIX%/}"
  object_key="${prefix}/${object_key}"
fi
pg_dump "${DATABASE_URL}" | gzip > "${tmp_file}"
if [ -n "${S3_ENDPOINT:-}" ]; then
  aws s3 cp "${tmp_file}" "s3://${S3_BUCKET}/${object_key}" --endpoint-url "${S3_ENDPOINT}"
else
  aws s3 cp "${tmp_file}" "s3://${S3_BUCKET}/${object_key}"
fi`,
									},
									Env: env,
									SecurityContext: &corev1.SecurityContext{
										AllowPrivilegeEscalation: boolPtr(false),
										RunAsNonRoot:             boolPtr(false),
									},
								},
							},
						},
					},
				},
			},
		},
	}

	if err := controllerutil.SetControllerReference(mint, cronJob, scheme); err != nil {
		return nil, fmt.Errorf("failed to set controller reference: %w", err)
	}

	return cronJob, nil
}

// GeneratePostgresRestoreJob creates a one-shot Job for restoring PostgreSQL from S3-compatible storage.
func GeneratePostgresRestoreJob(mint *mintv1alpha1.CashuMint, scheme *runtime.Scheme) (*batchv1.Job, error) {
	if mint.Spec.Backup == nil || !mint.Spec.Backup.Enabled {
		return nil, nil
	}
	if mint.Spec.Database.Engine != "postgres" ||
		mint.Spec.Database.Postgres == nil ||
		!mint.Spec.Database.Postgres.AutoProvision ||
		mint.Spec.Backup.S3 == nil {
		return nil, nil
	}
	if mint.Annotations == nil {
		return nil, nil
	}

	restoreObjectKey := strings.TrimSpace(mint.Annotations[RestoreObjectKeyAnnotation])
	if restoreObjectKey == "" {
		return nil, nil
	}

	restoreRequestID := strings.TrimSpace(mint.Annotations[RestoreRequestIDAnnotation])
	if restoreRequestID == "" {
		restoreRequestID = restoreObjectKey
	}
	jobNameSeed := restoreRequestID + ":" + restoreObjectKey

	labels := map[string]string{
		"app.kubernetes.io/name":       "cashu-mint",
		"app.kubernetes.io/instance":   mint.Name,
		"app.kubernetes.io/component":  "backup",
		"app.kubernetes.io/managed-by": "cashu-operator",
	}

	pgVersion := "15"
	if mint.Spec.Database.Postgres.AutoProvisionSpec != nil &&
		mint.Spec.Database.Postgres.AutoProvisionSpec.Version != "" {
		pgVersion = mint.Spec.Database.Postgres.AutoProvisionSpec.Version
	}

	env := []corev1.EnvVar{
		{
			Name: "DATABASE_URL",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: mint.Name + "-postgres-secret",
					},
					Key: "database-url",
				},
			},
		},
		{
			Name:  "S3_BUCKET",
			Value: mint.Spec.Backup.S3.Bucket,
		},
		{
			Name:  "RESTORE_OBJECT_KEY",
			Value: restoreObjectKey,
		},
		{
			Name: "AWS_ACCESS_KEY_ID",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &mint.Spec.Backup.S3.AccessKeyIDSecretRef,
			},
		},
		{
			Name: "AWS_SECRET_ACCESS_KEY",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &mint.Spec.Backup.S3.SecretAccessKeySecretRef,
			},
		},
	}

	if mint.Spec.Backup.S3.Region != "" {
		env = append(env, corev1.EnvVar{
			Name:  "AWS_DEFAULT_REGION",
			Value: mint.Spec.Backup.S3.Region,
		})
	}
	if mint.Spec.Backup.S3.Endpoint != "" {
		env = append(env, corev1.EnvVar{
			Name:  "S3_ENDPOINT",
			Value: mint.Spec.Backup.S3.Endpoint,
		})
	}

	backoffLimit := int32(1)
	ttlSeconds := int32(86400)
	job := &batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "batch/v1",
			Kind:       "Job",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-restore-%s", mint.Name, shortHash(jobNameSeed)),
			Namespace: mint.Namespace,
			Labels:    labels,
			Annotations: map[string]string{
				RestoreObjectKeyAnnotation: restoreObjectKey,
				RestoreRequestIDAnnotation: restoreRequestID,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			TTLSecondsAfterFinished: &ttlSeconds,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Containers: []corev1.Container{
						{
							Name:  "postgres-restore",
							Image: fmt.Sprintf("postgres:%s-alpine", pgVersion),
							Command: []string{
								"sh",
								"-ec",
								`set -euo pipefail
apk add --no-cache aws-cli >/dev/null
tmp_file="/tmp/restore.sql.gz"
if [ -n "${S3_ENDPOINT:-}" ]; then
  aws s3 cp "s3://${S3_BUCKET}/${RESTORE_OBJECT_KEY}" "${tmp_file}" --endpoint-url "${S3_ENDPOINT}"
else
  aws s3 cp "s3://${S3_BUCKET}/${RESTORE_OBJECT_KEY}" "${tmp_file}"
fi
gunzip -c "${tmp_file}" | psql "${DATABASE_URL}"`,
							},
							Env: env,
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: boolPtr(false),
								RunAsNonRoot:             boolPtr(false),
							},
						},
					},
				},
			},
		},
	}

	if err := controllerutil.SetControllerReference(mint, job, scheme); err != nil {
		return nil, fmt.Errorf("failed to set controller reference: %w", err)
	}

	return job, nil
}

func shortHash(input string) string {
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:])[:10]
}
