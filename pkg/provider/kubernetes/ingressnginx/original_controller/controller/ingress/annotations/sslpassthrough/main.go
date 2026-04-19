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

package sslpassthrough

import (
	networking "k8s.io/api/networking/v1"

	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original_controller/controller/ingress/annotations/parser"
	ing_errors "github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original_controller/controller/ingress/errors"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original_controller/controller/ingress/resolver"
)

const (
	sslPassthroughAnnotation = "ssl-passthrough"
)

var sslPassthroughAnnotations = parser.Annotation{
	Group: "", // TBD
	Annotations: parser.AnnotationFields{
		sslPassthroughAnnotation: {
			Validator:     parser.ValidateBool,
			Scope:         parser.AnnotationScopeIngress,
			Risk:          parser.AnnotationRiskLow, // Low, as it allows regexes but on a very limited set
			Documentation: `This annotation instructs the controller to send TLS connections directly to the backend instead of letting NGINX decrypt the communication.`,
		},
	},
}

type sslpt struct {
	r                resolver.Resolver
	annotationConfig parser.Annotation
}

// NewParser creates a new SSL passthrough annotation parser
func NewParser(r resolver.Resolver) parser.IngressAnnotation {
	return sslpt{
		r:                r,
		annotationConfig: sslPassthroughAnnotations,
	}
}

// ParseAnnotations parses the annotations contained in the ingress
// rule used to indicate if is required to configure
func (a sslpt) Parse(ing *networking.Ingress) (interface{}, error) {
	if ing.GetAnnotations() == nil {
		return false, ing_errors.ErrMissingAnnotations
	}

	return parser.GetBoolAnnotation(sslPassthroughAnnotation, ing, a.annotationConfig.Annotations)
}

func (a sslpt) GetDocumentation() parser.AnnotationFields {
	return a.annotationConfig.Annotations
}

func (a sslpt) Validate(anns map[string]string) error {
	maxrisk := parser.StringRiskToRisk(a.r.GetSecurityConfiguration().AnnotationsRiskLevel)
	return parser.CheckAnnotationRisk(anns, maxrisk, sslPassthroughAnnotations.Annotations)
}
