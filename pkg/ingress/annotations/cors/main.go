/*
Copyright 2016 The Kubernetes Authors.

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

package cors

import (
	"regexp"

	extensions "k8s.io/api/extensions/v1beta1"

	"k8s.io/ingress-nginx/pkg/ingress/annotations/parser"
)

const (
	annotationCorsEnabled          = "ingress.kubernetes.io/enable-cors"
	annotationCorsAllowOrigin      = "ingress.kubernetes.io/cors-allow-origin"
	annotationCorsAllowMethods     = "ingress.kubernetes.io/cors-allow-methods"
	annotationCorsAllowHeaders     = "ingress.kubernetes.io/cors-allow-headers"
	annotationCorsAllowCredentials = "ingress.kubernetes.io/cors-allow-credentials"
	// Default values
	defaultCorsMethods = "GET, PUT, POST, DELETE, PATCH, OPTIONS"
	defaultCorsHeaders = "DNT,X-CustomHeader,Keep-Alive,User-Agent,X-Requested-With,If-Modified-Since,Cache-Control,Content-Type,Authorization"
)

var (
	// Regex are defined here to prevent information leak, if user tries to set anything not valid
	// that could cause the Response to contain some internal value/variable (like returning $pid, $upstream_addr, etc)
	// Origin must contain a http/s Origin (including or not the port) or the value '*'
	corsOriginRegex = regexp.MustCompile(`^(https?://[A-Za-z0-9\-\.]*(:[0-9]+)?|\*)?$`)
	// Method must contain valid methods list (PUT, GET, POST, BLA)
	// May contain or not spaces between each verb
	corsMethodsRegex = regexp.MustCompile(`^([A-Za-z]+,?\s?)+$`)
	// Headers must contain valid values only (X-HEADER12, X-ABC)
	// May contain or not spaces between each Header
	corsHeadersRegex = regexp.MustCompile(`^([A-Za-z0-9\-\_]+,?\s?)+$`)
)

type cors struct {
}

// CorsConfig contains the Cors configuration to be used in the Ingress
type CorsConfig struct {
	CorsEnabled          bool   `json:"corsEnabled"`
	CorsAllowOrigin      string `json:"corsAllowOrigin"`
	CorsAllowMethods     string `json:"corsAllowMethods"`
	CorsAllowHeaders     string `json:"corsAllowHeaders"`
	CorsAllowCredentials bool   `json:"corsAllowCredentials"`
}

// NewParser creates a new CORS annotation parser
func NewParser() parser.IngressAnnotation {
	return cors{}
}

// Equal tests for equality between two External types
func (c1 *CorsConfig) Equal(c2 *CorsConfig) bool {
	if c1 == c2 {
		return true
	}
	if c1 == nil || c2 == nil {
		return false
	}
	if c1.CorsAllowCredentials != c2.CorsAllowCredentials {
		return false
	}
	if c1.CorsAllowHeaders != c2.CorsAllowHeaders {
		return false
	}
	if c1.CorsAllowMethods != c2.CorsAllowMethods {
		return false
	}
	if c1.CorsAllowOrigin != c2.CorsAllowOrigin {
		return false
	}
	if c1.CorsEnabled != c2.CorsEnabled {
		return false
	}

	return true
}

// Parse parses the annotations contained in the ingress
// rule used to indicate if the location/s should allows CORS
func (a cors) Parse(ing *extensions.Ingress) (interface{}, error) {
	corsenabled, err := parser.GetBoolAnnotation(annotationCorsEnabled, ing)
	if err != nil {
		corsenabled = false
	}

	corsalloworigin, err := parser.GetStringAnnotation(annotationCorsAllowOrigin, ing)
	if err != nil || corsalloworigin == "" || !corsOriginRegex.MatchString(corsalloworigin) {
		corsalloworigin = "*"
	}

	corsallowheaders, err := parser.GetStringAnnotation(annotationCorsAllowHeaders, ing)
	if err != nil || corsallowheaders == "" || !corsHeadersRegex.MatchString(corsallowheaders) {
		corsallowheaders = defaultCorsHeaders
	}

	corsallowmethods, err := parser.GetStringAnnotation(annotationCorsAllowMethods, ing)
	if err != nil || corsallowmethods == "" || !corsMethodsRegex.MatchString(corsallowmethods) {
		corsallowmethods = defaultCorsMethods
	}

	corsallowcredentials, err := parser.GetBoolAnnotation(annotationCorsAllowCredentials, ing)
	if err != nil {
		corsallowcredentials = true
	}

	return &CorsConfig{
		CorsEnabled:          corsenabled,
		CorsAllowOrigin:      corsalloworigin,
		CorsAllowHeaders:     corsallowheaders,
		CorsAllowMethods:     corsallowmethods,
		CorsAllowCredentials: corsallowcredentials,
	}, nil

}
