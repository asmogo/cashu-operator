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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Helper Functions", func() {
	Context("calculateConfigHash", func() {
		It("should return empty string for nil configmap", func() {
			Expect(calculateConfigHash(nil)).To(Equal(""))
		})

		It("should return empty string for nil data", func() {
			cm := &corev1.ConfigMap{}
			Expect(calculateConfigHash(cm)).To(Equal(""))
		})

		It("should return SHA256 of empty input for empty data", func() {
			cm := &corev1.ConfigMap{Data: map[string]string{}}
			// SHA256 of empty string
			Expect(calculateConfigHash(cm)).To(Equal("e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"))
		})

		It("should return a deterministic hash", func() {
			cm := &corev1.ConfigMap{
				Data: map[string]string{
					"config.toml": "[info]\nurl = \"http://test.local\"\n",
				},
			}
			h1 := calculateConfigHash(cm)
			h2 := calculateConfigHash(cm)
			Expect(h1).To(Equal(h2))
			Expect(h1).To(HaveLen(64)) // SHA256 hex = 64 chars
		})

		It("should produce different hashes for different configs", func() {
			cm1 := &corev1.ConfigMap{Data: map[string]string{"config.toml": "a"}}
			cm2 := &corev1.ConfigMap{Data: map[string]string{"config.toml": "b"}}
			Expect(calculateConfigHash(cm1)).NotTo(Equal(calculateConfigHash(cm2)))
		})

		It("should only hash config.toml key", func() {
			cm := &corev1.ConfigMap{
				Data: map[string]string{
					"other-key": "some-data",
				},
			}
			// No config.toml key means SHA256 of empty input
			Expect(calculateConfigHash(cm)).To(Equal("e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"))
		})
	})

	Context("isDeploymentReady", func() {
		It("should return false for nil deployment", func() {
			Expect(isDeploymentReady(nil)).To(BeFalse())
		})

		It("should return false when replicas is nil", func() {
			dep := &appsv1.Deployment{}
			Expect(isDeploymentReady(dep)).To(BeFalse())
		})

		It("should return false when not all replicas are ready", func() {
			replicas := int32(3)
			dep := &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{Replicas: &replicas},
				Status: appsv1.DeploymentStatus{
					ReadyReplicas:     2,
					UpdatedReplicas:   3,
					AvailableReplicas: 2,
				},
			}
			Expect(isDeploymentReady(dep)).To(BeFalse())
		})

		It("should return false when not all replicas are updated", func() {
			replicas := int32(3)
			dep := &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{Replicas: &replicas},
				Status: appsv1.DeploymentStatus{
					ReadyReplicas:     3,
					UpdatedReplicas:   2,
					AvailableReplicas: 3,
				},
			}
			Expect(isDeploymentReady(dep)).To(BeFalse())
		})

		It("should return false when not all replicas are available", func() {
			replicas := int32(3)
			dep := &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{Replicas: &replicas},
				Status: appsv1.DeploymentStatus{
					ReadyReplicas:     3,
					UpdatedReplicas:   3,
					AvailableReplicas: 2,
				},
			}
			Expect(isDeploymentReady(dep)).To(BeFalse())
		})

		It("should return true when all replicas are ready, updated, and available", func() {
			replicas := int32(3)
			dep := &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{Replicas: &replicas},
				Status: appsv1.DeploymentStatus{
					ReadyReplicas:     3,
					UpdatedReplicas:   3,
					AvailableReplicas: 3,
				},
			}
			Expect(isDeploymentReady(dep)).To(BeTrue())
		})

		It("should handle single replica", func() {
			replicas := int32(1)
			dep := &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{Replicas: &replicas},
				Status: appsv1.DeploymentStatus{
					ReadyReplicas:     1,
					UpdatedReplicas:   1,
					AvailableReplicas: 1,
				},
			}
			Expect(isDeploymentReady(dep)).To(BeTrue())
		})
	})

	Context("isStatefulSetReady", func() {
		It("should return false for nil statefulset", func() {
			Expect(isStatefulSetReady(nil)).To(BeFalse())
		})

		It("should return false when replicas is nil", func() {
			sts := &appsv1.StatefulSet{}
			Expect(isStatefulSetReady(sts)).To(BeFalse())
		})

		It("should return false when not ready", func() {
			replicas := int32(1)
			sts := &appsv1.StatefulSet{
				Spec: appsv1.StatefulSetSpec{Replicas: &replicas},
				Status: appsv1.StatefulSetStatus{
					ReadyReplicas:   0,
					CurrentReplicas: 1,
					UpdatedReplicas: 1,
				},
			}
			Expect(isStatefulSetReady(sts)).To(BeFalse())
		})

		It("should return true when all replicas are ready", func() {
			replicas := int32(1)
			sts := &appsv1.StatefulSet{
				Spec: appsv1.StatefulSetSpec{Replicas: &replicas},
				Status: appsv1.StatefulSetStatus{
					ReadyReplicas:   1,
					CurrentReplicas: 1,
					UpdatedReplicas: 1,
				},
			}
			Expect(isStatefulSetReady(sts)).To(BeTrue())
		})
	})
})
