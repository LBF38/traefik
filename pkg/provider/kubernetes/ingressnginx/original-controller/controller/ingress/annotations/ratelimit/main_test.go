/*
Copyright 2015 The Kubernetes Authors.

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

package ratelimit

import (
	"testing"

	api "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original-controller/controller/ingress/annotations/parser"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original-controller/controller/ingress/defaults"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original-controller/controller/ingress/errors"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original-controller/controller/ingress/resolver"
)

func buildIngress() *networking.Ingress {
	defaultBackend := networking.IngressBackend{
		Service: &networking.IngressServiceBackend{
			Name: "default-backend",
			Port: networking.ServiceBackendPort{
				Number: 80,
			},
		},
	}

	return &networking.Ingress{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:      "foo",
			Namespace: api.NamespaceDefault,
		},
		Spec: networking.IngressSpec{
			DefaultBackend: &networking.IngressBackend{
				Service: &networking.IngressServiceBackend{
					Name: "default-backend",
					Port: networking.ServiceBackendPort{
						Number: 80,
					},
				},
			},
			Rules: []networking.IngressRule{
				{
					Host: "foo.bar.com",
					IngressRuleValue: networking.IngressRuleValue{
						HTTP: &networking.HTTPIngressRuleValue{
							Paths: []networking.HTTPIngressPath{
								{
									Path:    "/foo",
									Backend: defaultBackend,
								},
							},
						},
					},
				},
			},
		},
	}
}

type mockBackend struct {
	resolver.Mock
}

func (m mockBackend) GetDefaultBackend() defaults.Backend {
	return defaults.Backend{
		LimitRateAfter: 0,
		LimitRate:      0,
	}
}

func TestWithoutAnnotations(t *testing.T) {
	ing := buildIngress()
	_, err := NewParser(mockBackend{}).Parse(ing)
	if err != nil && !errors.IsMissingAnnotations(err) {
		t.Errorf("unexpected error with ingress without annotations: %s", err)
	}
}

func TestRateLimiting(t *testing.T) {
	ing := buildIngress()

	data := map[string]string{}
	data[parser.GetAnnotationWithPrefix(limitRateConnectionsAnnotation)] = "0"
	data[parser.GetAnnotationWithPrefix(limitRateRPSAnnotation)] = "0"
	data[parser.GetAnnotationWithPrefix(limitRateRPMAnnotation)] = "0"
	ing.SetAnnotations(data)

	_, err := NewParser(mockBackend{}).Parse(ing)
	if err != nil {
		t.Errorf("unexpected error with invalid limits (0): %s", err)
	}

	data = map[string]string{}
	data[parser.GetAnnotationWithPrefix(limitRateConnectionsAnnotation)] = "5"
	data[parser.GetAnnotationWithPrefix(limitRateRPSAnnotation)] = "100"
	data[parser.GetAnnotationWithPrefix(limitRateRPMAnnotation)] = "10"
	data[parser.GetAnnotationWithPrefix(limitRateAfterAnnotation)] = "100"
	data[parser.GetAnnotationWithPrefix(limitRateAnnotation)] = "10"

	ing.SetAnnotations(data)

	i, err := NewParser(mockBackend{}).Parse(ing)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	rateLimit, ok := i.(*Config)
	if !ok {
		t.Errorf("expected a RateLimit type")
	}
	if rateLimit.Connections.Limit != 5 {
		t.Errorf("expected 5 in limit by ip but %v was returned", rateLimit.Connections)
	}
	if rateLimit.Connections.Burst != 5*5 {
		t.Errorf("expected %d in burst limit by ip but %v was returned", 5*3, rateLimit.Connections)
	}
	if rateLimit.RPS.Limit != 100 {
		t.Errorf("expected 100 in limit by rps but %v was returned", rateLimit.RPS)
	}
	if rateLimit.RPS.Burst != 100*5 {
		t.Errorf("expected %d in burst limit by rps but %v was returned", 100*3, rateLimit.RPS)
	}
	if rateLimit.RPM.Limit != 10 {
		t.Errorf("expected 10 in limit by rpm but %v was returned", rateLimit.RPM)
	}
	if rateLimit.RPM.Burst != 10*5 {
		t.Errorf("expected %d in burst limit by rpm but %v was returned", 10*3, rateLimit.RPM)
	}
	if rateLimit.LimitRateAfter != 100 {
		t.Errorf("expected 100 in limit by limitrateafter but %v was returned", rateLimit.LimitRateAfter)
	}
	if rateLimit.LimitRate != 10 {
		t.Errorf("expected 10 in limit by limitrate but %v was returned", rateLimit.LimitRate)
	}

	data = map[string]string{}
	data[parser.GetAnnotationWithPrefix(limitRateConnectionsAnnotation)] = "5"
	data[parser.GetAnnotationWithPrefix(limitRateRPSAnnotation)] = "100"
	data[parser.GetAnnotationWithPrefix(limitRateRPMAnnotation)] = "10"
	data[parser.GetAnnotationWithPrefix(limitRateAfterAnnotation)] = "100"
	data[parser.GetAnnotationWithPrefix(limitRateAnnotation)] = "10"
	data[parser.GetAnnotationWithPrefix(limitRateBurstMultiplierAnnotation)] = "3"

	ing.SetAnnotations(data)

	i, err = NewParser(mockBackend{}).Parse(ing)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	rateLimit, ok = i.(*Config)
	if !ok {
		t.Errorf("expected a RateLimit type")
	}
	if rateLimit.Connections.Limit != 5 {
		t.Errorf("expected 5 in limit by ip but %v was returned", rateLimit.Connections)
	}
	if rateLimit.Connections.Burst != 5*3 {
		t.Errorf("expected %d in burst limit by ip but %v was returned", 5*3, rateLimit.Connections)
	}
	if rateLimit.RPS.Limit != 100 {
		t.Errorf("expected 100 in limit by rps but %v was returned", rateLimit.RPS)
	}
	if rateLimit.RPS.Burst != 100*3 {
		t.Errorf("expected %d in burst limit by rps but %v was returned", 100*3, rateLimit.RPS)
	}
	if rateLimit.RPM.Limit != 10 {
		t.Errorf("expected 10 in limit by rpm but %v was returned", rateLimit.RPM)
	}
	if rateLimit.RPM.Burst != 10*3 {
		t.Errorf("expected %d in burst limit by rpm but %v was returned", 10*3, rateLimit.RPM)
	}
	if rateLimit.LimitRateAfter != 100 {
		t.Errorf("expected 100 in limit by limitrateafter but %v was returned", rateLimit.LimitRateAfter)
	}
	if rateLimit.LimitRate != 10 {
		t.Errorf("expected 10 in limit by limitrate but %v was returned", rateLimit.LimitRate)
	}
}

func TestAnnotationCIDR(t *testing.T) {
	ing := buildIngress()

	data := map[string]string{}
	data[parser.GetAnnotationWithPrefix(limitRateConnectionsAnnotation)] = "5"
	data[parser.GetAnnotationWithPrefix(limitAllowlistAnnotation)] = "192.168.0.5, 192.168.50.32/24"
	ing.SetAnnotations(data)

	i, err := NewParser(mockBackend{}).Parse(ing)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	rateLimit, ok := i.(*Config)
	if !ok {
		t.Errorf("expected a RateLimit type")
	}
	if len(rateLimit.Allowlist) != 2 {
		t.Errorf("expected 2 cidrs in limit by ip but %v was returned", len(rateLimit.Allowlist))
	}

	data = map[string]string{}
	data[parser.GetAnnotationWithPrefix(limitRateConnectionsAnnotation)] = "5"
	data[parser.GetAnnotationWithPrefix(limitWhitelistAnnotation)] = "192.168.0.5, 192.168.50.32/24, 10.10.10.1"
	ing.SetAnnotations(data)

	i, err = NewParser(mockBackend{}).Parse(ing)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	rateLimit, ok = i.(*Config)
	if !ok {
		t.Errorf("expected a RateLimit type")
	}
	if len(rateLimit.Allowlist) != 3 {
		t.Errorf("expected 3 cidrs in limit by ip but %v was returned", len(rateLimit.Allowlist))
	}

	// Parent annotation surpasses any alias
	data = map[string]string{}
	data[parser.GetAnnotationWithPrefix(limitRateConnectionsAnnotation)] = "5"
	data[parser.GetAnnotationWithPrefix(limitWhitelistAnnotation)] = "192.168.0.5, 192.168.50.32/24, 10.10.10.1"
	data[parser.GetAnnotationWithPrefix(limitAllowlistAnnotation)] = "192.168.0.9"
	ing.SetAnnotations(data)

	i, err = NewParser(mockBackend{}).Parse(ing)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	rateLimit, ok = i.(*Config)
	if !ok {
		t.Errorf("expected a RateLimit type")
	}
	if len(rateLimit.Allowlist) != 1 {
		t.Errorf("expected 1 cidrs in limit by ip but %v was returned", len(rateLimit.Allowlist))
	}
}
