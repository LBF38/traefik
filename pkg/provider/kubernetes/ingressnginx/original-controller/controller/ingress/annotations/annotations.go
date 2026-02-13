/*
Copyright 2017 The Kubernetes Authors.

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
	"dario.cat/mergo"

	apiv1 "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/controller/ingress/annotations/alias"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/controller/ingress/annotations/auth"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/controller/ingress/annotations/authreq"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/controller/ingress/annotations/authreqglobal"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/controller/ingress/annotations/authtls"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/controller/ingress/annotations/backendprotocol"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/controller/ingress/annotations/canary"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/controller/ingress/annotations/clientbodybuffersize"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/controller/ingress/annotations/connection"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/controller/ingress/annotations/cors"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/controller/ingress/annotations/customheaders"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/controller/ingress/annotations/customhttperrors"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/controller/ingress/annotations/defaultbackend"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/controller/ingress/annotations/disableproxyintercepterrors"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/controller/ingress/annotations/fastcgi"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/controller/ingress/annotations/http2pushpreload"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/controller/ingress/annotations/ipallowlist"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/controller/ingress/annotations/ipdenylist"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/controller/ingress/annotations/loadbalancing"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/controller/ingress/annotations/log"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/controller/ingress/annotations/mirror"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/controller/ingress/annotations/modsecurity"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/controller/ingress/annotations/opentelemetry"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/controller/ingress/annotations/parser"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/controller/ingress/annotations/portinredirect"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/controller/ingress/annotations/proxy"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/controller/ingress/annotations/proxyssl"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/controller/ingress/annotations/ratelimit"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/controller/ingress/annotations/redirect"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/controller/ingress/annotations/rewrite"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/controller/ingress/annotations/satisfy"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/controller/ingress/annotations/serversnippet"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/controller/ingress/annotations/serviceupstream"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/controller/ingress/annotations/sessionaffinity"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/controller/ingress/annotations/snippet"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/controller/ingress/annotations/sslcipher"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/controller/ingress/annotations/sslpassthrough"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/controller/ingress/annotations/streamsnippet"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/controller/ingress/annotations/upstreamhashby"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/controller/ingress/annotations/upstreamvhost"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/controller/ingress/annotations/xforwardedprefix"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/controller/ingress/errors"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/controller/ingress/resolver"
)

// DeniedKeyName name of the key that contains the reason to deny a location
const DeniedKeyName = "Denied"

// Ingress defines the valid annotations present in one NGINX Ingress rule
type Ingress struct {
	metav1.ObjectMeta
	BackendProtocol             string
	Aliases                     []string
	BasicDigestAuth             auth.Config
	Canary                      canary.Config
	CertificateAuth             authtls.Config
	ClientBodyBufferSize        string
	CustomHeaders               customheaders.Config
	ConfigurationSnippet        string
	Connection                  connection.Config
	CorsConfig                  cors.Config
	CustomHTTPErrors            []int
	DisableProxyInterceptErrors bool
	DefaultBackend              *apiv1.Service
	FastCGI                     fastcgi.Config
	Denied                      *string
	ExternalAuth                authreq.Config
	EnableGlobalAuth            bool
	HTTP2PushPreload            bool
	Opentelemetry               opentelemetry.Config
	Proxy                       proxy.Config
	ProxySSL                    proxyssl.Config
	RateLimit                   ratelimit.Config
	Redirect                    redirect.Config
	Rewrite                     rewrite.Config
	Satisfy                     string
	ServerSnippet               string
	ServiceUpstream             bool
	SessionAffinity             sessionaffinity.Config
	SSLPassthrough              bool
	UsePortInRedirects          bool
	UpstreamHashBy              upstreamhashby.Config
	LoadBalancing               string
	UpstreamVhost               string
	Denylist                    ipdenylist.SourceRange
	XForwardedPrefix            string
	SSLCipher                   sslcipher.Config
	Logs                        log.Config
	ModSecurity                 modsecurity.Config
	Mirror                      mirror.Config
	StreamSnippet               string
	Allowlist                   ipallowlist.SourceRange
}

// Extractor defines the annotation parsers to be used in the extraction of annotations
type Extractor struct {
	annotations map[string]parser.IngressAnnotation
}

func NewAnnotationFactory(cfg resolver.Resolver) map[string]parser.IngressAnnotation {
	return map[string]parser.IngressAnnotation{
		"Aliases":                     alias.NewParser(cfg),
		"BasicDigestAuth":             auth.NewParser(auth.AuthDirectory, cfg),
		"Canary":                      canary.NewParser(cfg),
		"CertificateAuth":             authtls.NewParser(cfg),
		"ClientBodyBufferSize":        clientbodybuffersize.NewParser(cfg),
		"CustomHeaders":               customheaders.NewParser(cfg),
		"ConfigurationSnippet":        snippet.NewParser(cfg),
		"Connection":                  connection.NewParser(cfg),
		"CorsConfig":                  cors.NewParser(cfg),
		"CustomHTTPErrors":            customhttperrors.NewParser(cfg),
		"DisableProxyInterceptErrors": disableproxyintercepterrors.NewParser(cfg),
		"DefaultBackend":              defaultbackend.NewParser(cfg),
		"FastCGI":                     fastcgi.NewParser(cfg),
		"ExternalAuth":                authreq.NewParser(cfg),
		"EnableGlobalAuth":            authreqglobal.NewParser(cfg),
		"HTTP2PushPreload":            http2pushpreload.NewParser(cfg),
		"Opentelemetry":               opentelemetry.NewParser(cfg),
		"Proxy":                       proxy.NewParser(cfg),
		"ProxySSL":                    proxyssl.NewParser(cfg),
		"RateLimit":                   ratelimit.NewParser(cfg),
		"Redirect":                    redirect.NewParser(cfg),
		"Rewrite":                     rewrite.NewParser(cfg),
		"Satisfy":                     satisfy.NewParser(cfg),
		"ServerSnippet":               serversnippet.NewParser(cfg),
		"ServiceUpstream":             serviceupstream.NewParser(cfg),
		"SessionAffinity":             sessionaffinity.NewParser(cfg),
		"SSLPassthrough":              sslpassthrough.NewParser(cfg),
		"UsePortInRedirects":          portinredirect.NewParser(cfg),
		"UpstreamHashBy":              upstreamhashby.NewParser(cfg),
		"LoadBalancing":               loadbalancing.NewParser(cfg),
		"UpstreamVhost":               upstreamvhost.NewParser(cfg),
		"Allowlist":                   ipallowlist.NewParser(cfg),
		"Denylist":                    ipdenylist.NewParser(cfg),
		"XForwardedPrefix":            xforwardedprefix.NewParser(cfg),
		"SSLCipher":                   sslcipher.NewParser(cfg),
		"Logs":                        log.NewParser(cfg),
		"BackendProtocol":             backendprotocol.NewParser(cfg),
		"ModSecurity":                 modsecurity.NewParser(cfg),
		"Mirror":                      mirror.NewParser(cfg),
		"StreamSnippet":               streamsnippet.NewParser(cfg),
	}
}

// NewAnnotationExtractor creates a new annotations extractor
func NewAnnotationExtractor(cfg resolver.Resolver) Extractor {
	return Extractor{
		NewAnnotationFactory(cfg),
	}
}

// Extract extracts the annotations from an Ingress
func (e Extractor) Extract(ing *networking.Ingress) (*Ingress, error) {
	pia := &Ingress{
		ObjectMeta: ing.ObjectMeta,
	}

	data := make(map[string]interface{})
	for name, annotationParser := range e.annotations {
		if err := annotationParser.Validate(ing.GetAnnotations()); err != nil {
			return nil, errors.NewRiskyAnnotations(name)
		}
		val, err := annotationParser.Parse(ing)
		klog.V(5).InfoS("Parsing Ingress annotation", "name", name, "ingress", klog.KObj(ing), "value", val)
		if err != nil {
			if errors.IsValidationError(err) {
				klog.ErrorS(err, "ingress contains invalid annotation value")
				return nil, err
			}
			if errors.IsMissingAnnotations(err) {
				continue
			}

			if !errors.IsLocationDenied(err) {
				continue
			}

			if name == "CertificateAuth" && data[name] == nil {
				data[name] = authtls.Config{
					AuthTLSError: err.Error(),
				}
				// avoid mapping the result from the annotation
				val = nil
			}

			_, alreadyDenied := data[DeniedKeyName]
			if !alreadyDenied {
				errString := err.Error()
				data[DeniedKeyName] = &errString
				klog.ErrorS(err, "error reading Ingress annotation", "name", name, "ingress", klog.KObj(ing))
				continue
			}

			klog.V(5).ErrorS(err, "error reading Ingress annotation", "name", name, "ingress", klog.KObj(ing))
		}

		if val != nil {
			data[name] = val
		}
	}

	err := mergo.MapWithOverwrite(pia, data)
	if err != nil {
		klog.ErrorS(err, "unexpected error merging extracted annotations")
	}

	return pia, nil
}
