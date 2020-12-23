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
	"net/http"
	"strings"

	"github.com/onsi/ginkgo"
	"k8s.io/ingress-nginx/test/e2e/framework"
)

var _ = framework.DescribeAnnotation("modsecurity owasp", func() {
	f := framework.NewDefaultFramework("modsecuritylocation")

	ginkgo.BeforeEach(func() {
		f.NewEchoDeployment()
	})

	ginkgo.It("should enable modsecurity", func() {
		host := "modsecurity.foo.com"
		nameSpace := f.Namespace

		annotations := map[string]string{
			"nginx.ingress.kubernetes.io/enable-modsecurity": "true",
		}

		ing := framework.NewSingleIngress(host, "/", host, nameSpace, framework.EchoService, 80, annotations)
		f.EnsureIngress(ing)

		f.WaitForNginxServer(host,
			func(server string) bool {
				return strings.Contains(server, "modsecurity on;") &&
					strings.Contains(server, "modsecurity_rules_file /etc/nginx/modsecurity/modsecurity.conf;")
			})
	})

	ginkgo.It("should enable modsecurity with transaction ID and OWASP rules", func() {
		host := "modsecurity.foo.com"
		nameSpace := f.Namespace

		annotations := map[string]string{
			"nginx.ingress.kubernetes.io/enable-modsecurity":         "true",
			"nginx.ingress.kubernetes.io/enable-owasp-core-rules":    "true",
			"nginx.ingress.kubernetes.io/modsecurity-transaction-id": "modsecurity-$request_id",
		}

		ing := framework.NewSingleIngress(host, "/", host, nameSpace, framework.EchoService, 80, annotations)
		f.EnsureIngress(ing)

		f.WaitForNginxServer(host,
			func(server string) bool {
				return strings.Contains(server, "modsecurity on;") &&
					strings.Contains(server, "modsecurity_rules_file /etc/nginx/owasp-modsecurity-crs/nginx-modsecurity.conf;") &&
					strings.Contains(server, "modsecurity_transaction_id \"modsecurity-$request_id\";")
			})
	})

	ginkgo.It("should disable modsecurity", func() {
		host := "modsecurity.foo.com"
		nameSpace := f.Namespace

		annotations := map[string]string{
			"nginx.ingress.kubernetes.io/enable-modsecurity": "false",
		}

		ing := framework.NewSingleIngress(host, "/", host, nameSpace, framework.EchoService, 80, annotations)
		f.EnsureIngress(ing)

		f.WaitForNginxServer(host,
			func(server string) bool {
				return !strings.Contains(server, "modsecurity on;")
			})
	})

	ginkgo.It("should enable modsecurity with snippet", func() {
		host := "modsecurity.foo.com"
		nameSpace := f.Namespace

		annotations := map[string]string{
			"nginx.ingress.kubernetes.io/enable-modsecurity":  "true",
			"nginx.ingress.kubernetes.io/modsecurity-snippet": "SecRuleEngine On",
		}

		ing := framework.NewSingleIngress(host, "/", host, nameSpace, framework.EchoService, 80, annotations)
		f.EnsureIngress(ing)

		f.WaitForNginxServer(host,
			func(server string) bool {
				return strings.Contains(server, "modsecurity on;") &&
					strings.Contains(server, "SecRuleEngine On")
			})
	})

	ginkgo.It("should enable modsecurity without using 'modsecurity on;'", func() {
		f.SetNginxConfigMapData(map[string]string{
			"enable-modsecurity": "true"},
		)

		host := "modsecurity.foo.com"
		nameSpace := f.Namespace

		annotations := map[string]string{
			"nginx.ingress.kubernetes.io/enable-modsecurity": "true",
		}

		ing := framework.NewSingleIngress(host, "/", host, nameSpace, framework.EchoService, 80, annotations)
		f.EnsureIngress(ing)

		f.WaitForNginxServer(host,
			func(server string) bool {
				return !strings.Contains(server, "modsecurity on;") &&
					!strings.Contains(server, "modsecurity_rules_file /etc/nginx/modsecurity/modsecurity.conf;")
			})
	})

	ginkgo.It("should disable modsecurity using 'modsecurity off;'", func() {
		f.SetNginxConfigMapData(map[string]string{
			"enable-modsecurity": "true"},
		)

		host := "modsecurity.foo.com"
		nameSpace := f.Namespace

		annotations := map[string]string{
			"nginx.ingress.kubernetes.io/enable-modsecurity": "false",
		}

		ing := framework.NewSingleIngress(host, "/", host, nameSpace, framework.EchoService, 80, annotations)
		f.EnsureIngress(ing)

		f.WaitForNginxServer(host,
			func(server string) bool {
				return strings.Contains(server, "modsecurity off;")
			})
	})

	ginkgo.It("should enable modsecurity with snippet and block requests", func() {
		host := "modsecurity.foo.com"
		nameSpace := f.Namespace

		snippet := `SecRuleEngine On
		SecRequestBodyAccess On
		SecAuditEngine RelevantOnly
		SecAuditLogParts ABIJDEFHZ
		SecAuditLog /dev/stdout
		SecAuditLogType Serial
		SecRule REQUEST_HEADERS:User-Agent \"block-ua\" \"log,deny,id:107,status:403,msg:\'UA blocked\'\"`

		annotations := map[string]string{
			"nginx.ingress.kubernetes.io/enable-modsecurity":  "true",
			"nginx.ingress.kubernetes.io/modsecurity-snippet": snippet,
		}

		ing := framework.NewSingleIngress(host, "/", host, nameSpace, framework.EchoService, 80, annotations)
		f.EnsureIngress(ing)

		f.WaitForNginxServer(host,
			func(server string) bool {
				return strings.Contains(server, "modsecurity on;") &&
					strings.Contains(server, "SecRuleEngine On")
			})

		f.HTTPTestClient().
			GET("/").
			WithHeader("Host", host).
			WithHeader("User-Agent", "block-ua").
			Expect().
			Status(http.StatusForbidden)
	})

	ginkgo.It("should enable modsecurity globally and with modsecurity-snippet block requests", func() {
		host := "modsecurity.foo.com"
		nameSpace := f.Namespace

		snippet := `SecRuleEngine On
		SecRequestBodyAccess On
		SecAuditEngine RelevantOnly
		SecAuditLogParts ABIJDEFHZ
		SecAuditLog /dev/stdout
		SecAuditLogType Serial
		SecRule REQUEST_HEADERS:User-Agent \"block-ua\" \"log,deny,id:107,status:403,msg:\'UA blocked\'\"`

		annotations := map[string]string{
			"nginx.ingress.kubernetes.io/modsecurity-snippet": snippet,
		}

		ing := framework.NewSingleIngress(host, "/", host, nameSpace, framework.EchoService, 80, annotations)
		f.EnsureIngress(ing)

		f.UpdateNginxConfigMapData("enable-modsecurity", "true")

		f.WaitForNginxServer(host,
			func(server string) bool {
				return strings.Contains(server, "SecRuleEngine On")
			})

		f.HTTPTestClient().
			GET("/").
			WithHeader("Host", host).
			WithHeader("User-Agent", "block-ua").
			Expect().
			Status(http.StatusForbidden)
	})

	ginkgo.It("trying to move modsec conf to nginx conf", func() {
		host := "modsecurity.foo.com"
		nameSpace := f.Namespace

		snippet := `SecAuditLogParts ABIJDEFHZ`

		annotations := map[string]string{
			"nginx.ingress.kubernetes.io/modsecurity-snippet": snippet,
		}

		ing := framework.NewSingleIngress(host, "/", host, nameSpace, framework.EchoService, 80, annotations)
		f.EnsureIngress(ing)

		f.UpdateNginxConfigMapData("enable-modsecurity", "true")
		f.UpdateNginxConfigMapData("SecRuleEngine", "On")
		f.UpdateNginxConfigMapData("SecRequestBodyAccess", "RelevantOnly")
		f.UpdateNginxConfigMapData("SecAuditLogParts", "ABIJDEFHZ")
		f.UpdateNginxConfigMapData("SecAuditLog", "/dev/stdout")
		f.UpdateNginxConfigMapData("SecAuditLogType", "Serial")
		f.UpdateNginxConfigMapData("SecRule", "REQUEST_HEADERS:User-Agent \"block-ua\" \"log,deny,id:107,status:403,msg:'UA blocked'\"")



		f.WaitForNginxServer(host,
			func(server string) bool {
				return strings.Contains(server, "SecRuleEngine On")
			})

		f.HTTPTestClient().
			GET("/").
			WithHeader("Host", host).
			WithHeader("User-Agent", "block-ua").
			Expect().
			Status(http.StatusForbidden)
	})



})
