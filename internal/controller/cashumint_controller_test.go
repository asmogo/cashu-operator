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

package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	mintv1alpha1 "github.com/asmogo/cashu-operator/api/v1alpha1"
	"github.com/asmogo/cashu-operator/internal/controller/generators"
)

// fakeRecorder is a simple event recorder for tests
type fakeRecorder struct{}

func (f *fakeRecorder) Event(object runtime.Object, eventtype, reason, message string) {}
func (f *fakeRecorder) Eventf(object runtime.Object, eventtype, reason, messageFmt string, args ...interface{}) {
}
func (f *fakeRecorder) AnnotatedEventf(object runtime.Object, annotations map[string]string, eventtype, reason, messageFmt string, args ...interface{}) {
}

var _ = Describe("CashuMint Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		cashumint := &mintv1alpha1.CashuMint{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind CashuMint")
			err := k8sClient.Get(ctx, typeNamespacedName, cashumint)
			if err != nil && errors.IsNotFound(err) {
				resource := &mintv1alpha1.CashuMint{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: mintv1alpha1.CashuMintSpec{
						MintInfo: mintv1alpha1.MintInfo{
							URL: "http://test-mint.local",
						},
						Database: mintv1alpha1.DatabaseConfig{
							Engine: "sqlite",
							SQLite: &mintv1alpha1.SQLiteConfig{
								DataDir: "/data",
							},
						},
						Lightning: mintv1alpha1.LightningConfig{
							Backend: "fakewallet",
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &mintv1alpha1.CashuMint{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance CashuMint")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &CashuMintReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: &fakeRecorder{},
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})

	Context("When rollout dependencies are missing", func() {
		const resourceName = "test-missing-secret"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		BeforeEach(func() {
			By("creating the custom resource with a missing required Secret reference")
			resource := &mintv1alpha1.CashuMint{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: mintv1alpha1.CashuMintSpec{
					MintInfo: mintv1alpha1.MintInfo{
						URL: "http://test-mint.local",
					},
					Database: mintv1alpha1.DatabaseConfig{
						Engine: "postgres",
						Postgres: &mintv1alpha1.PostgresConfig{
							URLSecretRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "external-db-secret",
								},
								Key: "database-url",
							},
						},
					},
					Lightning: mintv1alpha1.LightningConfig{
						Backend: "fakewallet",
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
		})

		AfterEach(func() {
			resource := &mintv1alpha1.CashuMint{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if errors.IsNotFound(err) {
				return
			}
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should reconcile without blocking when Secrets are missing (Kubernetes handles missing secrets at runtime)", func() {
			controllerReconciler := &CashuMintReconciler{
				Client:   k8sClient,
				Recorder: &fakeRecorder{},
				Scheme:   k8sClient.Scheme(),
			}

			var result reconcile.Result
			var err error
			for i := 0; i < 3; i++ {
				result, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})
				Expect(err).NotTo(HaveOccurred())
			}

			// The simplified reconciler no longer gates on missing secrets;
			// it proceeds and lets Kubernetes surface missing-secret errors at pod start.
			Expect(result.RequeueAfter).To(BeNumerically(">", 0))

			updated := &mintv1alpha1.CashuMint{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, updated)).To(Succeed())
			// Config should be generated and a Deployment created
			configMap := &corev1.ConfigMap{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: resourceName + "-config", Namespace: "default"}, configMap)).To(Succeed())
		})
	})

	Context("When auto-provisioned PostgreSQL is not ready", func() {
		const resourceName = "test-postgres-gating"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		BeforeEach(func() {
			By("creating the custom resource with auto-provisioned PostgreSQL")
			resource := &mintv1alpha1.CashuMint{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: mintv1alpha1.CashuMintSpec{
					MintInfo: mintv1alpha1.MintInfo{
						URL: "http://test-mint.local",
					},
					Database: mintv1alpha1.DatabaseConfig{
						Engine: "postgres",
						Postgres: &mintv1alpha1.PostgresConfig{
							AutoProvision: true,
						},
					},
					Lightning: mintv1alpha1.LightningConfig{
						Backend: "fakewallet",
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
		})

		AfterEach(func() {
			resource := &mintv1alpha1.CashuMint{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if errors.IsNotFound(err) {
				return
			}
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should provision PostgreSQL and proceed with deployment (no longer gates on postgres readiness)", func() {
			controllerReconciler := &CashuMintReconciler{
				Client:   k8sClient,
				Recorder: &fakeRecorder{},
				Scheme:   k8sClient.Scheme(),
			}

			var result reconcile.Result
			var err error
			for i := 0; i < 3; i++ {
				result, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})
				Expect(err).NotTo(HaveOccurred())
			}

			// The simplified reconciler no longer gates on postgres readiness;
			// it provisions postgres resources and continues with deployment.
			Expect(result.RequeueAfter).To(BeNumerically(">", 0))

			updated := &mintv1alpha1.CashuMint{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, updated)).To(Succeed())

			// PostgreSQL StatefulSet should be created
			postgresStatefulSet := &appsv1.StatefulSet{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      resourceName + "-postgres",
				Namespace: "default",
			}, postgresStatefulSet)).To(Succeed())

			// DatabaseReady condition should be set
			dbCondition := meta.FindStatusCondition(updated.Status.Conditions, mintv1alpha1.ConditionTypeDatabaseReady)
			Expect(dbCondition).NotTo(BeNil())
		})
	})

	Context("When backup restore is requested", func() {
		const (
			resourceName      = "test-backup-restore"
			restoreObjectName = "cashumint-production/cashumint-production-20250101T000000Z.sql.gz"
		)

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		backupSecretName := types.NamespacedName{
			Name:      "backup-credentials",
			Namespace: "default",
		}

		BeforeEach(func() {
			By("creating backup credentials and a custom resource with restore annotations")
			backupSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      backupSecretName.Name,
					Namespace: backupSecretName.Namespace,
				},
				StringData: map[string]string{
					"AWS_ACCESS_KEY_ID":     "test-access-key",
					"AWS_SECRET_ACCESS_KEY": "test-secret-key",
				},
			}
			Expect(k8sClient.Create(ctx, backupSecret)).To(Succeed())

			resource := &mintv1alpha1.CashuMint{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
					Annotations: map[string]string{
						generators.RestoreObjectKeyAnnotation: restoreObjectName,
						generators.RestoreRequestIDAnnotation: "manual-restore-1",
					},
				},
				Spec: mintv1alpha1.CashuMintSpec{
					MintInfo: mintv1alpha1.MintInfo{
						URL: "http://test-mint.local",
					},
					Database: mintv1alpha1.DatabaseConfig{
						Engine: "postgres",
						Postgres: &mintv1alpha1.PostgresConfig{
							AutoProvision: true,
						},
					},
					Backup: &mintv1alpha1.BackupConfig{
						Enabled:  true,
						Schedule: "0 */6 * * *",
						S3: &mintv1alpha1.S3BackupConfig{
							Bucket: "mint-backups",
							AccessKeyIDSecretRef: corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: backupSecretName.Name},
								Key:                  "AWS_ACCESS_KEY_ID",
							},
							SecretAccessKeySecretRef: corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: backupSecretName.Name},
								Key:                  "AWS_SECRET_ACCESS_KEY",
							},
						},
					},
					Lightning: mintv1alpha1.LightningConfig{
						Backend: "fakewallet",
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
		})

		AfterEach(func() {
			resource := &mintv1alpha1.CashuMint{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			} else {
				Expect(errors.IsNotFound(err)).To(BeTrue())
			}

			backupSecret := &corev1.Secret{}
			err = k8sClient.Get(ctx, backupSecretName, backupSecret)
			if err == nil {
				Expect(k8sClient.Delete(ctx, backupSecret)).To(Succeed())
			} else {
				Expect(errors.IsNotFound(err)).To(BeTrue())
			}
		})

		It("should reconcile backup and restore resources with backup status condition", func() {
			controllerReconciler := &CashuMintReconciler{
				Client:   k8sClient,
				Recorder: &fakeRecorder{},
				Scheme:   k8sClient.Scheme(),
			}

			for i := 0; i < 4; i++ {
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})
				Expect(err).NotTo(HaveOccurred())
			}

			cronJob := &batchv1.CronJob{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      resourceName + "-backup",
				Namespace: "default",
			}, cronJob)).To(Succeed())

			jobList := &batchv1.JobList{}
			Expect(k8sClient.List(ctx, jobList,
				client.InNamespace("default"),
				client.MatchingLabels{
					"app.kubernetes.io/instance":  resourceName,
					"app.kubernetes.io/component": "backup",
				},
			)).To(Succeed())
			Expect(jobList.Items).To(HaveLen(1))
			Expect(jobList.Items[0].Name).To(ContainSubstring(resourceName + "-restore-"))
			Expect(jobList.Items[0].Annotations[generators.RestoreObjectKeyAnnotation]).To(Equal(restoreObjectName))

			updated := &mintv1alpha1.CashuMint{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, updated)).To(Succeed())

			backupCondition := meta.FindStatusCondition(updated.Status.Conditions, mintv1alpha1.ConditionTypeBackupReady)
			Expect(backupCondition).NotTo(BeNil())
			Expect(backupCondition.Status).To(Equal(metav1.ConditionTrue))
			Expect(backupCondition.Reason).To(Equal("RestoreJobReconciled"))
		})
	})

	Context("When external PostgreSQL URL is secret-backed", func() {
		const resourceName = "test-external-db-secret-env"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		dbSecretName := types.NamespacedName{
			Name:      "external-db-secret-env",
			Namespace: "default",
		}

		BeforeEach(func() {
			By("creating the external database URL secret and custom resource")
			dbSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      dbSecretName.Name,
					Namespace: dbSecretName.Namespace,
				},
				StringData: map[string]string{
					"database-url": "postgresql://user:pass@db:5432/cdk?sslmode=require",
				},
			}
			Expect(k8sClient.Create(ctx, dbSecret)).To(Succeed())

			resource := &mintv1alpha1.CashuMint{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: mintv1alpha1.CashuMintSpec{
					MintInfo: mintv1alpha1.MintInfo{
						URL: "http://test-mint.local",
					},
					Database: mintv1alpha1.DatabaseConfig{
						Engine: "postgres",
						Postgres: &mintv1alpha1.PostgresConfig{
							URLSecretRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: dbSecretName.Name},
								Key:                  "database-url",
							},
						},
					},
					Lightning: mintv1alpha1.LightningConfig{
						Backend: "fakewallet",
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
		})

		AfterEach(func() {
			resource := &mintv1alpha1.CashuMint{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			} else {
				Expect(errors.IsNotFound(err)).To(BeTrue())
			}

			dbSecret := &corev1.Secret{}
			err = k8sClient.Get(ctx, dbSecretName, dbSecret)
			if err == nil {
				Expect(k8sClient.Delete(ctx, dbSecret)).To(Succeed())
			} else {
				Expect(errors.IsNotFound(err)).To(BeTrue())
			}
		})

		It("should set deployment database URL from SecretKeyRef", func() {
			controllerReconciler := &CashuMintReconciler{
				Client:   k8sClient,
				Recorder: &fakeRecorder{},
				Scheme:   k8sClient.Scheme(),
			}

			for i := 0; i < 3; i++ {
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})
				Expect(err).NotTo(HaveOccurred())
			}

			deployment := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, deployment)).To(Succeed())
			Expect(deployment.Spec.Template.Spec.Containers).NotTo(BeEmpty())

			var dbEnv *corev1.EnvVar
			for i := range deployment.Spec.Template.Spec.Containers[0].Env {
				envVar := &deployment.Spec.Template.Spec.Containers[0].Env[i]
				if envVar.Name == "CDK_MINTD_DATABASE_URL" {
					dbEnv = envVar
					break
				}
			}

			Expect(dbEnv).NotTo(BeNil())
			Expect(dbEnv.Value).To(BeEmpty())
			Expect(dbEnv.ValueFrom).NotTo(BeNil())
			Expect(dbEnv.ValueFrom.SecretKeyRef).NotTo(BeNil())
			Expect(dbEnv.ValueFrom.SecretKeyRef.Name).To(Equal(dbSecretName.Name))
			Expect(dbEnv.ValueFrom.SecretKeyRef.Key).To(Equal("database-url"))
		})
	})

	Context("When a required Secret key is missing", func() {
		const resourceName = "test-missing-secret-key"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		dbSecretName := types.NamespacedName{
			Name:      "external-db-secret-key-missing",
			Namespace: "default",
		}

		BeforeEach(func() {
			By("creating a secret without the referenced database key")
			dbSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      dbSecretName.Name,
					Namespace: dbSecretName.Namespace,
				},
				StringData: map[string]string{
					"other-key": "postgresql://user:pass@db:5432/cdk?sslmode=require",
				},
			}
			Expect(k8sClient.Create(ctx, dbSecret)).To(Succeed())

			resource := &mintv1alpha1.CashuMint{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: mintv1alpha1.CashuMintSpec{
					MintInfo: mintv1alpha1.MintInfo{
						URL: "http://test-mint.local",
					},
					Database: mintv1alpha1.DatabaseConfig{
						Engine: "postgres",
						Postgres: &mintv1alpha1.PostgresConfig{
							URLSecretRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: dbSecretName.Name},
								Key:                  "database-url",
							},
						},
					},
					Lightning: mintv1alpha1.LightningConfig{
						Backend: "fakewallet",
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
		})

		AfterEach(func() {
			resource := &mintv1alpha1.CashuMint{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			} else {
				Expect(errors.IsNotFound(err)).To(BeTrue())
			}

			dbSecret := &corev1.Secret{}
			err = k8sClient.Get(ctx, dbSecretName, dbSecret)
			if err == nil {
				Expect(k8sClient.Delete(ctx, dbSecret)).To(Succeed())
			} else {
				Expect(errors.IsNotFound(err)).To(BeTrue())
			}
		})

		It("should reconcile without blocking when a Secret key is missing (Kubernetes handles this at pod start)", func() {
			controllerReconciler := &CashuMintReconciler{
				Client:   k8sClient,
				Recorder: &fakeRecorder{},
				Scheme:   k8sClient.Scheme(),
			}

			var result reconcile.Result
			var err error
			for i := 0; i < 3; i++ {
				result, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})
				Expect(err).NotTo(HaveOccurred())
			}

			// The simplified reconciler no longer gates on missing secret keys;
			// it proceeds and lets Kubernetes surface the error at pod start.
			Expect(result.RequeueAfter).To(BeNumerically(">", 0))

			updated := &mintv1alpha1.CashuMint{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, updated)).To(Succeed())

			// Config and Deployment should be created
			configMap := &corev1.ConfigMap{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: resourceName + "-config", Namespace: "default"}, configMap)).To(Succeed())
		})
	})

	Context("When backup is enabled but cannot be reconciled", func() {
		const resourceName = "test-backup-not-reconciled"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		dbSecretName := types.NamespacedName{
			Name:      "external-db-secret-backup",
			Namespace: "default",
		}

		backupSecretName := types.NamespacedName{
			Name:      "backup-credentials-not-reconciled",
			Namespace: "default",
		}

		BeforeEach(func() {
			By("creating external database and backup credentials secrets")
			dbSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      dbSecretName.Name,
					Namespace: dbSecretName.Namespace,
				},
				StringData: map[string]string{
					"database-url": "postgresql://user:pass@db:5432/cdk?sslmode=require",
				},
			}
			Expect(k8sClient.Create(ctx, dbSecret)).To(Succeed())

			backupSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      backupSecretName.Name,
					Namespace: backupSecretName.Namespace,
				},
				StringData: map[string]string{
					"AWS_ACCESS_KEY_ID":     "test-access-key",
					"AWS_SECRET_ACCESS_KEY": "test-secret-key",
				},
			}
			Expect(k8sClient.Create(ctx, backupSecret)).To(Succeed())

			resource := &mintv1alpha1.CashuMint{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: mintv1alpha1.CashuMintSpec{
					MintInfo: mintv1alpha1.MintInfo{
						URL: "http://test-mint.local",
					},
					Database: mintv1alpha1.DatabaseConfig{
						Engine: "postgres",
						Postgres: &mintv1alpha1.PostgresConfig{
							URLSecretRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: dbSecretName.Name},
								Key:                  "database-url",
							},
						},
					},
					Backup: &mintv1alpha1.BackupConfig{
						Enabled: true,
						S3: &mintv1alpha1.S3BackupConfig{
							Bucket: "mint-backups",
							AccessKeyIDSecretRef: corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: backupSecretName.Name},
								Key:                  "AWS_ACCESS_KEY_ID",
							},
							SecretAccessKeySecretRef: corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: backupSecretName.Name},
								Key:                  "AWS_SECRET_ACCESS_KEY",
							},
						},
					},
					Lightning: mintv1alpha1.LightningConfig{
						Backend: "fakewallet",
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
		})

		AfterEach(func() {
			resource := &mintv1alpha1.CashuMint{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			} else {
				Expect(errors.IsNotFound(err)).To(BeTrue())
			}

			dbSecret := &corev1.Secret{}
			err = k8sClient.Get(ctx, dbSecretName, dbSecret)
			if err == nil {
				Expect(k8sClient.Delete(ctx, dbSecret)).To(Succeed())
			} else {
				Expect(errors.IsNotFound(err)).To(BeTrue())
			}

			backupSecret := &corev1.Secret{}
			err = k8sClient.Get(ctx, backupSecretName, backupSecret)
			if err == nil {
				Expect(k8sClient.Delete(ctx, backupSecret)).To(Succeed())
			} else {
				Expect(errors.IsNotFound(err)).To(BeTrue())
			}
		})

		It("should set backup status to not reconciled without creating cronjob", func() {
			controllerReconciler := &CashuMintReconciler{
				Client:   k8sClient,
				Recorder: &fakeRecorder{},
				Scheme:   k8sClient.Scheme(),
			}

			for i := 0; i < 3; i++ {
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})
				Expect(err).NotTo(HaveOccurred())
			}

			cronJob := &batchv1.CronJob{}
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      resourceName + "-backup",
				Namespace: "default",
			}, cronJob)
			Expect(errors.IsNotFound(err)).To(BeTrue())

			deployment := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, deployment)).To(Succeed())

			updated := &mintv1alpha1.CashuMint{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, updated)).To(Succeed())

			backupCondition := meta.FindStatusCondition(updated.Status.Conditions, mintv1alpha1.ConditionTypeBackupReady)
			Expect(backupCondition).NotTo(BeNil())
			Expect(backupCondition.Status).To(Equal(metav1.ConditionFalse))
			Expect(backupCondition.Reason).To(Equal("BackupNotReconciled"))
		})
	})

	// The "When using managed payment processors" Context was removed because
	// standalone PaymentProcessors are no longer supported. Use the generic
	// sidecar (spec.lightning.grpcProcessor.sidecarProcessor) instead.
})
