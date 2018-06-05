/*
Copyright 2018 The Kubernetes Authors.

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

package ctrl_test

import (
	"github.com/kubernetes-sigs/kubebuilder/pkg/ctrl"
	"github.com/kubernetes-sigs/kubebuilder/pkg/ctrl/eventhandler"
	"github.com/kubernetes-sigs/kubebuilder/pkg/ctrl/reconcile"
	"github.com/kubernetes-sigs/kubebuilder/pkg/ctrl/source"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Controller", func() {
	var c chan reconcile.ReconcileRequest
	var stop chan struct{}

	BeforeEach(func() {
		stop = make(chan struct{})
		c = make(chan reconcile.ReconcileRequest)
		Expect(config).NotTo(BeNil())
	})

	AfterEach(func() {
		close(stop)
	})

	Describe("Controller", func() {
		It("should Reconcile", func(done Done) {
			By("Creating the Controller and adding it to the ControllerManager")
			instance := &ctrl.Controller{
				Reconcile: reconcile.ReconcileFunc(
					func(r reconcile.ReconcileRequest) (reconcile.ReconcileResult, error) {
						c <- r
						return reconcile.ReconcileResult{}, nil
					}),
			}
			cm := &ctrl.ControllerManager{
				Config: config,
			}
			cm.AddController(instance, func() {
				// Deferred from the Init function
				instance.Watch(&source.KindSource{Type: &appsv1.ReplicaSet{}}, &eventhandler.EnqueueOwnerHandler{
					OwnerType: &appsv1.Deployment{},
				})
				instance.Watch(&source.KindSource{Type: &appsv1.Deployment{}}, &eventhandler.EnqueueHandler{})
			})

			By("Starting the ControllerManager")
			go func() {
				defer GinkgoRecover()
				err := cm.Start(stop)
				Expect(err).NotTo(HaveOccurred())
			}()

			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "deployment-name"},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"foo": "bar"},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"foo": "bar"}},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "nginx",
									Image: "nginx",
								},
							},
						},
					},
				},
			}
			expectedReconcileRequest := reconcile.ReconcileRequest{NamespacedName: types.NamespacedName{
				Namespace: "default",
				Name:      "deployment-name",
			}}

			By("Invoking Reconciling for Create")
			_, err := clientset.AppsV1().Deployments("default").Create(deployment)
			Expect(err).NotTo(HaveOccurred())
			Expect(<-c).To(Equal(expectedReconcileRequest))

			By("Invoking Reconciling for Update")
			newDeployment := deployment.DeepCopy()
			newDeployment.Labels = map[string]string{"foo": "bar"}
			_, err = clientset.AppsV1().Deployments("default").Update(newDeployment)
			Expect(err).NotTo(HaveOccurred())
			Expect(<-c).To(Equal(expectedReconcileRequest))

			By("Invoking Reconciling for Delete")
			err = clientset.AppsV1().Deployments("default").
				Delete("deployment-name", &metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(<-c).To(Equal(expectedReconcileRequest))

			close(done)
		}, 5)
	})
})