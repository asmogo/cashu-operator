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
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	mintv1alpha1 "github.com/asmogo/cashu-operator/api/v1alpha1"
)

func backupMint(name string) *mintv1alpha1.CashuMint {
	return &mintv1alpha1.CashuMint{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: mintv1alpha1.CashuMintSpec{
			MintInfo: mintv1alpha1.MintInfo{URL: "http://test.local"},
			Database: mintv1alpha1.DatabaseConfig{
				Engine:   "postgres",
				Postgres: &mintv1alpha1.PostgresConfig{AutoProvision: true},
			},
			PaymentBackend: mintv1alpha1.PaymentBackendConfig{FakeWallet: &mintv1alpha1.FakeWalletConfig{}},
			Backup: &mintv1alpha1.BackupConfig{
				Enabled:  true,
				Schedule: "0 */6 * * *",
				S3: &mintv1alpha1.S3BackupConfig{
					Bucket: "my-backups",
					AccessKeyIDSecretRef: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "backup-creds"},
						Key:                  "aws_access_key_id",
					},
					SecretAccessKeySecretRef: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "backup-creds"},
						Key:                  "aws_secret_access_key",
					},
				},
			},
		},
	}
}

func TestGeneratePostgresBackupCronJob_Disabled(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("no-backup")

	cj, err := GeneratePostgresBackupCronJob(mint, scheme)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cj != nil {
		t.Errorf("expected nil for disabled backup, got %v", cj)
	}
}

func TestGeneratePostgresBackupCronJob_NonPostgres(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("sqlite-backup")
	mint.Spec.Backup = &mintv1alpha1.BackupConfig{Enabled: true}

	cj, err := GeneratePostgresBackupCronJob(mint, scheme)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cj != nil {
		t.Errorf("expected nil for sqlite backup, got %v", cj)
	}
}

func TestGeneratePostgresBackupCronJob_NoS3(t *testing.T) {
	scheme := testScheme(t)
	mint := &mintv1alpha1.CashuMint{
		ObjectMeta: metav1.ObjectMeta{Name: "no-s3", Namespace: "default"},
		Spec: mintv1alpha1.CashuMintSpec{
			Database: mintv1alpha1.DatabaseConfig{
				Engine:   "postgres",
				Postgres: &mintv1alpha1.PostgresConfig{AutoProvision: true},
			},
			PaymentBackend: mintv1alpha1.PaymentBackendConfig{FakeWallet: &mintv1alpha1.FakeWalletConfig{}},
			Backup:         &mintv1alpha1.BackupConfig{Enabled: true},
		},
	}

	cj, err := GeneratePostgresBackupCronJob(mint, scheme)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cj != nil {
		t.Errorf("expected nil for no S3 config, got %v", cj)
	}
}

func TestGeneratePostgresBackupCronJob_FullConfig(t *testing.T) {
	scheme := testScheme(t)
	mint := backupMint("bk-mint")

	cj, err := GeneratePostgresBackupCronJob(mint, scheme)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cj == nil {
		t.Fatal("expected CronJob, got nil")
	}
	if cj.Name != "bk-mint-backup" {
		t.Errorf("name = %q, want %q", cj.Name, "bk-mint-backup")
	}
	if cj.Spec.Schedule != "0 */6 * * *" {
		t.Errorf("schedule = %q, want %q", cj.Spec.Schedule, "0 */6 * * *")
	}
	if *cj.Spec.SuccessfulJobsHistoryLimit != 14 {
		t.Errorf("historyLimit = %d, want 14", *cj.Spec.SuccessfulJobsHistoryLimit)
	}

	containers := cj.Spec.JobTemplate.Spec.Template.Spec.Containers
	if len(containers) != 1 || containers[0].Name != "postgres-backup" {
		t.Fatalf("expected postgres-backup container")
	}
	assertContains(t, containers[0].Image, "postgres:15-alpine")

	// Check env vars
	envMap := map[string]bool{}
	for _, e := range containers[0].Env {
		envMap[e.Name] = true
	}
	for _, name := range []string{"DATABASE_URL", "MINT_NAME", "S3_BUCKET", "AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY"} {
		if !envMap[name] {
			t.Errorf("missing env var %q", name)
		}
	}

	assertLabelsContain(t, cj.Labels, "app.kubernetes.io/component", "backup")
}

func TestGeneratePostgresBackupCronJob_CustomRetention(t *testing.T) {
	scheme := testScheme(t)
	mint := backupMint("custom-bk")
	mint.Spec.Backup.RetentionCount = int32Ptr(30)

	cj, _ := GeneratePostgresBackupCronJob(mint, scheme)
	if *cj.Spec.SuccessfulJobsHistoryLimit != 30 {
		t.Errorf("historyLimit = %d, want 30", *cj.Spec.SuccessfulJobsHistoryLimit)
	}
}

func TestGeneratePostgresBackupCronJob_S3RegionEndpoint(t *testing.T) {
	scheme := testScheme(t)
	mint := backupMint("s3-opts")
	mint.Spec.Backup.S3.Region = "us-east-1"
	mint.Spec.Backup.S3.Endpoint = "https://s3.custom.endpoint"

	cj, _ := GeneratePostgresBackupCronJob(mint, scheme)
	envMap := map[string]string{}
	for _, e := range cj.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Env {
		envMap[e.Name] = e.Value
	}
	if envMap["AWS_DEFAULT_REGION"] != "us-east-1" {
		t.Errorf("region = %q, want %q", envMap["AWS_DEFAULT_REGION"], "us-east-1")
	}
	if envMap["S3_ENDPOINT"] != "https://s3.custom.endpoint" {
		t.Errorf("endpoint = %q, want %q", envMap["S3_ENDPOINT"], "https://s3.custom.endpoint")
	}
}

func TestGeneratePostgresBackupCronJob_CustomPgVersion(t *testing.T) {
	scheme := testScheme(t)
	mint := backupMint("pgver-bk")
	mint.Spec.Database.Postgres.AutoProvisionSpec = &mintv1alpha1.PostgresAutoProvisionSpec{Version: "16"}

	cj, _ := GeneratePostgresBackupCronJob(mint, scheme)
	assertContains(t, cj.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Image, "postgres:16-alpine")
}

// --- Restore Job tests ---

func TestGeneratePostgresRestoreJob_NoAnnotations(t *testing.T) {
	scheme := testScheme(t)
	mint := backupMint("no-ann")

	job, err := GeneratePostgresRestoreJob(mint, scheme)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if job != nil {
		t.Errorf("expected nil Job without restore annotations, got %v", job)
	}
}

func TestGeneratePostgresRestoreJob_EmptyAnnotationValue(t *testing.T) {
	scheme := testScheme(t)
	mint := backupMint("empty-ann")
	mint.Annotations = map[string]string{
		RestoreObjectKeyAnnotation: "",
	}

	job, _ := GeneratePostgresRestoreJob(mint, scheme)
	if job != nil {
		t.Errorf("expected nil Job with empty annotation, got %v", job)
	}
}

func TestGeneratePostgresRestoreJob_WithAnnotations(t *testing.T) {
	scheme := testScheme(t)
	mint := backupMint("restore-mint")
	mint.Annotations = map[string]string{
		RestoreObjectKeyAnnotation: "backup-2025.sql.gz",
		RestoreRequestIDAnnotation: "request-123",
	}

	job, err := GeneratePostgresRestoreJob(mint, scheme)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if job == nil {
		t.Fatal("expected Job, got nil")
	}
	assertContains(t, job.Name, "restore-mint-restore-")
	if job.Annotations[RestoreObjectKeyAnnotation] != "backup-2025.sql.gz" {
		t.Error("restore object key annotation missing")
	}
	if job.Annotations[RestoreRequestIDAnnotation] != "request-123" {
		t.Error("restore request ID annotation missing")
	}

	containers := job.Spec.Template.Spec.Containers
	if len(containers) != 1 || containers[0].Name != "postgres-restore" {
		t.Fatal("expected postgres-restore container")
	}

	envMap := map[string]string{}
	for _, e := range containers[0].Env {
		envMap[e.Name] = e.Value
	}
	if envMap["RESTORE_OBJECT_KEY"] != "backup-2025.sql.gz" {
		t.Errorf("RESTORE_OBJECT_KEY = %q, want %q", envMap["RESTORE_OBJECT_KEY"], "backup-2025.sql.gz")
	}
}

func TestGeneratePostgresRestoreJob_Disabled(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("no-restore")
	mint.Annotations = map[string]string{
		RestoreObjectKeyAnnotation: "backup.sql.gz",
	}

	job, _ := GeneratePostgresRestoreJob(mint, scheme)
	if job != nil {
		t.Errorf("expected nil for disabled backup, got %v", job)
	}
}
