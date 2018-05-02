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

package annotations

import (
	"fmt"
	"net/http"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/parnurzeal/gorequest"

	"k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"k8s.io/ingress-nginx/test/e2e/framework"
)

var _ = framework.IngressNginxDescribe("Annotations - ProxyPass", func() {
	f := framework.NewDefaultFramework("proxypass")

	BeforeEach(func() {
		err := f.NewEchoDeployment()
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
	})

	It("should use the defined proxypass address instead of the normal nginx upstream", func() {
		host := "example.com"
		proxyPass := fmt.Sprintf("http-svc.%s.svc.cluster.local", f.IngressController.Namespace)

		ing, err := f.EnsureIngress(&v1beta1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      host,
				Namespace: f.IngressController.Namespace,
				Annotations: map[string]string{
					"nginx.ingress.kubernetes.io/proxy-pass-address": proxyPass,
					"nginx.ingress.kubernetes.io/proxy-pass-port":    "80",
				},
			},
			Spec: v1beta1.IngressSpec{
				Rules: []v1beta1.IngressRule{
					{
						Host: host,
						IngressRuleValue: v1beta1.IngressRuleValue{
							HTTP: &v1beta1.HTTPIngressRuleValue{
								Paths: []v1beta1.HTTPIngressPath{
									{
										Path: "/",
										Backend: v1beta1.IngressBackend{
											ServiceName: "http-svc",
											ServicePort: intstr.FromInt(80),
										},
									},
								},
							},
						},
					},
				},
			},
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(ing).NotTo(BeNil())

		err = f.WaitForNginxServer(host,
			func(server string) bool {
				return strings.Contains(server, "proxy_pass http://"+proxyPass+":80;")
			})
		Expect(err).NotTo(HaveOccurred(), "should set custom proxy_pass instead of using upstream address")

		resp, body, errs := gorequest.New().
			Get(f.IngressController.HTTPURL).
			Set("Host", host).
			End()

		Expect(len(errs)).Should(BeNumerically("==", 0))
		Expect(resp.StatusCode).Should(Equal(http.StatusOK), fmt.Sprintf("should have status code %d on response, got %d", http.StatusOK, resp.StatusCode))
		Expect(body).Should(ContainSubstring(fmt.Sprintf("host=%v", host)))
	})

	It("should redirect to '/something' with defined proxypass address", func() {
		host := "example.com"
		proxyPass := fmt.Sprintf("http-svc.%s.svc.cluster.local", f.IngressController.Namespace)

		ing, err := f.EnsureIngress(&v1beta1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      host,
				Namespace: f.IngressController.Namespace,
				Annotations: map[string]string{
					"nginx.ingress.kubernetes.io/proxy-pass-address": proxyPass,
					"nginx.ingress.kubernetes.io/proxy-pass-port":    "80",
					"nginx.ingress.kubernetes.io/rewrite-target":     "/something",
				},
			},
			Spec: v1beta1.IngressSpec{
				Rules: []v1beta1.IngressRule{
					{
						Host: host,
						IngressRuleValue: v1beta1.IngressRuleValue{
							HTTP: &v1beta1.HTTPIngressRuleValue{
								Paths: []v1beta1.HTTPIngressPath{
									{
										Path: "/",
										Backend: v1beta1.IngressBackend{
											ServiceName: "http-svc",
											ServicePort: intstr.FromInt(80),
										},
									},
								},
							},
						},
					},
				},
			},
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(ing).NotTo(BeNil())

		err = f.WaitForNginxServer(host,
			func(server string) bool {
				return strings.Contains(server, "proxy_pass http://"+proxyPass+":80;")
			})
		Expect(err).NotTo(HaveOccurred(), "should set custom proxy_pass instead of using upstream address")

		resp, body, errs := gorequest.New().
			Get(f.IngressController.HTTPURL).
			Set("Host", host).
			End()

		Expect(len(errs)).Should(BeNumerically("==", 0))
		Expect(resp.StatusCode).Should(Equal(http.StatusOK), fmt.Sprintf("should have status code %d on response, got %d", http.StatusOK, resp.StatusCode))
		Expect(body).Should(ContainSubstring(fmt.Sprintf("host=%v", host)))
	})
})
