package ingressnginxv2

import (
	"fmt"
	"net"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original_controller/controller/ingress/annotations/canary"
	logCfg "github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original_controller/controller/ingress/annotations/log"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original_controller/controller/ingress/annotations/proxy"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original_controller/controller/ingress/defaults"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original_controller/controller/k8s"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	netv1 "k8s.io/api/networking/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
)

const (
	rootLocation    = "/"
	defUpstreamName = "upstream-default-backend"
	defServerName   = "_"
	emptyZone       = ""
)

var (
	pathTypeExact  = netv1.PathTypeExact
	pathTypePrefix = netv1.PathTypePrefix
)

func (p *Provider) getNginxConfiguration(ingresses []*Ingress) (sets.Set[string], []*Server, *NginxConfiguration) {
	upstreams, servers := p.getBackendServers(ingresses)
	var passUpstreams []*SSLPassthroughBackend

	hosts := sets.New[string]()

	for _, server := range servers {
		// If a location is defined by a prefix string that ends with the slash character, and requests are processed by one of
		// proxy_pass, fastcgi_pass, uwsgi_pass, scgi_pass, memcached_pass, or grpc_pass, then the special processing is performed.
		// In response to a request with URI equal to // this string, but without the trailing slash, a permanent redirect with the
		// code 301 will be returned to the requested URI with the slash appended. If this is not desired, an exact match of the
		// URIand location could be defined like this:
		//
		// location /user/ {
		//     proxy_pass http://user.example.com;
		// }
		// location = /user {
		//     proxy_pass http://login.example.com;
		// }
		server.Locations = updateServerLocations(server.Locations)

		if !hosts.Has(server.Hostname) {
			hosts.Insert(server.Hostname)
		}

		for _, alias := range server.Aliases {
			if !hosts.Has(alias) {
				hosts.Insert(alias)
			}
		}

		if !server.SSLPassthrough {
			continue
		}

		for _, loc := range server.Locations {
			if loc.Path != rootLocation {
				log.Warn().Msgf("Ignoring SSL Passthrough for location %q in server %q", loc.Path, server.Hostname)
				continue
			}
			passUpstreams = append(passUpstreams, &SSLPassthroughBackend{
				Backend:  loc.Backend,
				Hostname: server.Hostname,
				Service:  loc.Service,
				Port:     loc.Port,
			})
			break
		}
	}

	return hosts, servers, &NginxConfiguration{
		Backends: upstreams,
		Servers:  servers,
		// TCPEndpoints:          n.getStreamServices(n.cfg.TCPConfigMapName, corev1.ProtocolTCP),
		// UDPEndpoints:          n.getStreamServices(n.cfg.UDPConfigMapName, corev1.ProtocolUDP),
		PassthroughBackends: passUpstreams,
		// BackendConfigChecksum: n.GetBackendConfiguration().Checksum,
		// DefaultSSLCertificate: n.getDefaultSSLCertificate(),
		// StreamSnippets:        n.getStreamSnippets(ingresses),
	}
}

// createUpstreams creates the NGINX upstreams (Endpoints) for each Service
// referenced in Ingress rules.
func (p *Provider) createUpstreams(data []*Ingress, du *Backend) map[string]*Backend {
	upstreams := make(map[string]*Backend)
	upstreams[defUpstreamName] = du

	for _, ing := range data {
		ingKey := k8s.MetaNamespaceKey(ing)
		anns := ing.ParsedAnnotationsNGINX

		if !p.AllowSnippetAnnotations {
			dropSnippetDirectives(anns, ingKey)
		}

		var defBackend string
		if ing.Spec.DefaultBackend != nil && ing.Spec.DefaultBackend.Service != nil {
			defBackend = upstreamName(ing.Namespace, ing.Spec.DefaultBackend.Service)

			log.Info().Msgf("Creating upstream %q", defBackend)
			upstreams[defBackend] = newUpstream(defBackend)

			upstreams[defBackend].UpstreamHashBy.UpstreamHashBy = anns.UpstreamHashBy.UpstreamHashBy
			upstreams[defBackend].UpstreamHashBy.UpstreamHashBySubset = anns.UpstreamHashBy.UpstreamHashBySubset
			upstreams[defBackend].UpstreamHashBy.UpstreamHashBySubsetSize = anns.UpstreamHashBy.UpstreamHashBySubsetSize

			upstreams[defBackend].LoadBalancing = anns.LoadBalancing
			if upstreams[defBackend].LoadBalancing == "" {
				upstreams[defBackend].LoadBalancing = p.LoadBalancing
			}

			svcKey := fmt.Sprintf("%v/%v", ing.Namespace, ing.Spec.DefaultBackend.Service.Name)

			// add the service ClusterIP as a single Endpoint instead of individual Endpoints
			if anns.ServiceUpstream {
				endpoint, err := p.getServiceClusterEndpoint(svcKey, ing.Spec.DefaultBackend)
				if err != nil {
					log.Error().Msgf("Failed to determine a suitable ClusterIP Endpoint for Service %q: %v", svcKey, err)
				} else {
					upstreams[defBackend].Endpoints = []Endpoint{endpoint}
				}
			}

			// configure traffic shaping for canary
			if anns.Canary.Enabled {
				upstreams[defBackend].NoServer = true
				upstreams[defBackend].TrafficShapingPolicy = newTrafficShapingPolicy(&anns.Canary)
			}

			if len(upstreams[defBackend].Endpoints) == 0 {
				_, port := upstreamServiceNameAndPort(ing.Spec.DefaultBackend.Service)
				endps, err := p.serviceEndpoints(svcKey, port.String())
				upstreams[defBackend].Endpoints = append(upstreams[defBackend].Endpoints, endps...)
				if err != nil {
					log.Warn().Msgf("Error creating upstream %q: %v", defBackend, err)
				}
			}

			s, err := p.GetService(svcKey)
			if err != nil {
				log.Warn().Msgf("Error obtaining Service %q: %v", svcKey, err)
			}
			upstreams[defBackend].Service = s
		}

		for _, rule := range ing.Spec.Rules {
			if rule.HTTP == nil {
				continue
			}

			for i, path := range rule.HTTP.Paths {
				if path.Backend.Service == nil {
					// skip non-service backends
					log.Info().Msgf("Ingress %q and path %q does not contain a service backend, using default backend", ingKey, path.Path)
					continue
				}

				name := upstreamName(ing.Namespace, path.Backend.Service)
				svcName, svcPort := upstreamServiceNameAndPort(path.Backend.Service)
				if _, ok := upstreams[name]; ok {
					continue
				}

				log.Info().Msgf("Creating upstream %q", name)
				upstreams[name] = newUpstream(name)
				upstreams[name].Port = svcPort

				upstreams[name].UpstreamHashBy.UpstreamHashBy = anns.UpstreamHashBy.UpstreamHashBy
				upstreams[name].UpstreamHashBy.UpstreamHashBySubset = anns.UpstreamHashBy.UpstreamHashBySubset
				upstreams[name].UpstreamHashBy.UpstreamHashBySubsetSize = anns.UpstreamHashBy.UpstreamHashBySubsetSize

				upstreams[name].LoadBalancing = anns.LoadBalancing
				if upstreams[name].LoadBalancing == "" {
					upstreams[name].LoadBalancing = p.LoadBalancing
				}

				svcKey := fmt.Sprintf("%v/%v", ing.Namespace, svcName)

				// add the service ClusterIP as a single Endpoint instead of individual Endpoints
				if anns.ServiceUpstream {
					endpoint, err := p.getServiceClusterEndpoint(svcKey, &rule.HTTP.Paths[i].Backend)
					if err != nil {
						log.Error().Msgf("Failed to determine a suitable ClusterIP Endpoint for Service %q: %v", svcKey, err)
					} else {
						upstreams[name].Endpoints = []Endpoint{endpoint}
					}
				}

				// configure traffic shaping for canary
				if anns.Canary.Enabled {
					upstreams[name].NoServer = true
					upstreams[name].TrafficShapingPolicy = newTrafficShapingPolicy(&anns.Canary)
				}

				if len(upstreams[name].Endpoints) == 0 {
					_, port := upstreamServiceNameAndPort(path.Backend.Service)
					endp, err := p.serviceEndpoints(svcKey, port.String())
					if err != nil {
						log.Warn().Msgf("Error obtaining Endpoints for Service %q: %v", svcKey, err)
						// p.metricCollector.IncOrphanIngress(ing.Namespace, ing.Name, orphanMetricLabelNoService)
						continue
					}
					// p.metricCollector.DecOrphanIngress(ing.Namespace, ing.Name, orphanMetricLabelNoService)

					// if len(endp) == 0 {
					// 	p.metricCollector.IncOrphanIngress(ing.Namespace, ing.Name, orphanMetricLabelNoEndpoint)
					// } else {
					// 	p.metricCollector.DecOrphanIngress(ing.Namespace, ing.Name, orphanMetricLabelNoEndpoint)
					// }
					upstreams[name].Endpoints = endp
				}

				s, err := p.GetService(svcKey)
				if err != nil {
					log.Warn().Msgf("Error obtaining Service %q: %v", svcKey, err)
					continue
				}

				upstreams[name].Service = s
			}
		}
	}

	return upstreams
}

// DefaultEndpoint returns the default endpoint to be use as default server that returns 404.
func (p *Provider) DefaultEndpoint() Endpoint {
	return Endpoint{
		Address: "127.0.0.1",
		// Port:    fmt.Sprintf("%v", n.ListenPorts.Default), // FIXME
		Port:   "80",
		Target: &corev1.ObjectReference{},
	}
}

func (p *Provider) GetService(svcKey string) (*corev1.Service, error) {
	res := strings.Split(svcKey, "/")
	return p.k8sClient.GetService(res[0], res[1])
}

// serviceEndpoints returns the upstream servers (Endpoints) associated with a Service.
func (p *Provider) serviceEndpoints(svcKey, backendPort string) ([]Endpoint, error) {
	var upstreams []Endpoint

	svc, err := p.GetService(svcKey)
	if err != nil {
		return upstreams, err
	}
	var zone string
	if p.EnableTopologyAwareRouting {
		zone = getIngressPodZone(svc)
	} else {
		zone = emptyZone
	}
	log.Info().Msgf("Obtaining ports information for Service %q", svcKey)
	// Ingress with an ExternalName Service and no port defined for that Service
	if svc.Spec.Type == corev1.ServiceTypeExternalName {
		if p.DisableServiceExternalName {
			log.Warn().Msgf("Service %q of type ExternalName not allowed due to Ingress configuration.", svcKey)
			return upstreams, nil
		}
		servicePort := externalNamePorts(backendPort, svc)
		endps := getEndpointsFromSlices(svc, servicePort, corev1.ProtocolTCP, zone, p.GetServiceEndpointsSlices)
		if len(endps) == 0 {
			log.Warn().Msgf("Service %q does not have any active Endpoint.", svcKey)
			return upstreams, nil
		}

		upstreams = append(upstreams, endps...)
		return upstreams, nil
	}

	for i := range svc.Spec.Ports {
		servicePort := svc.Spec.Ports[i]
		// targetPort could be a string, use either the port name or number (int)
		if strconv.Itoa(int(servicePort.Port)) == backendPort ||
			servicePort.TargetPort.String() == backendPort ||
			servicePort.Name == backendPort {
			endps := getEndpointsFromSlices(svc, &servicePort, corev1.ProtocolTCP, zone, p.GetServiceEndpointsSlices)
			if len(endps) == 0 {
				log.Warn().Msgf("Service %q does not have any active Endpoint.", svcKey)
			}

			upstreams = append(upstreams, endps...)
			break
		}
	}

	return upstreams, nil
}

// TODO: WIP
func (p *Provider) GetDefaultBackend() defaults.Backend {
	// TODO: FIXME
	return defaults.Backend{}
}

// createServers builds a map of host name to Server structs from a map of
// already computed Upstream structs. Each Server is configured with at least
// one root location, which uses a default backend if left unspecified.
func (p *Provider) createServers(data []*Ingress,
	upstreams map[string]*Backend,
	du *Backend,
) map[string]*Server {
	servers := make(map[string]*Server, len(data))
	allAliases := make(map[string][]string, len(data))

	bdef := p.GetDefaultBackend()
	ngxProxy := proxy.Config{
		BodySize:             bdef.ProxyBodySize,
		ConnectTimeout:       bdef.ProxyConnectTimeout,
		SendTimeout:          bdef.ProxySendTimeout,
		ReadTimeout:          bdef.ProxyReadTimeout,
		BuffersNumber:        bdef.ProxyBuffersNumber,
		BufferSize:           bdef.ProxyBufferSize,
		BusyBuffersSize:      bdef.ProxyBusyBuffersSize,
		CookieDomain:         bdef.ProxyCookieDomain,
		CookiePath:           bdef.ProxyCookiePath,
		NextUpstream:         bdef.ProxyNextUpstream,
		NextUpstreamTimeout:  bdef.ProxyNextUpstreamTimeout,
		NextUpstreamTries:    bdef.ProxyNextUpstreamTries,
		RequestBuffering:     bdef.ProxyRequestBuffering,
		ProxyRedirectFrom:    bdef.ProxyRedirectFrom,
		ProxyBuffering:       bdef.ProxyBuffering,
		ProxyHTTPVersion:     bdef.ProxyHTTPVersion,
		ProxyMaxTempFileSize: bdef.ProxyMaxTempFileSize,
	}

	// initialize default server and root location
	pathTypePrefix := netv1.PathTypePrefix
	servers[defServerName] = &Server{
		Hostname: defServerName,
		SSLCert:  p.getDefaultSSLCertificate(), // FIXME
		Locations: []*Location{
			{
				Path:         rootLocation,
				PathType:     &pathTypePrefix,
				IsDefBackend: true,
				Backend:      du.Name,
				Proxy:        ngxProxy,
				Service:      du.Service,
				Logs: logCfg.Config{
					Access:  p.EnableAccessLogForDefaultBackend,
					Rewrite: false,
				},
			},
		},
	}

	// initialize all other servers
	for _, ing := range data {
		ingKey := k8s.MetaNamespaceKey(ing)
		anns := toAnnotations(ing.ParsedAnnotations)

		if !p.AllowSnippetAnnotations {
			dropSnippetDirectives(anns, ingKey)
		}

		// default upstream name
		un := du.Name

		if anns.Canary.Enabled {
			log.Info().Msgf("Ingress %v is marked as Canary, ignoring", ingKey)
			continue
		}

		if ing.Spec.DefaultBackend != nil && ing.Spec.DefaultBackend.Service != nil {
			defUpstream := upstreamName(ing.Namespace, ing.Spec.DefaultBackend.Service)

			if backendUpstream, ok := upstreams[defUpstream]; ok {
				// use backend specified in Ingress as the default backend for all its rules
				un = backendUpstream.Name

				defLoc := servers[defServerName].Locations[0]
				if defLoc.IsDefBackend && len(ing.Spec.Rules) == 0 {
					log.Info().Msgf("Ingress %q defines a backend but no rule. Using it to configure the catch-all server %q", ingKey, defServerName)

					defLoc.IsDefBackend = false
					// special "catch all" case, Ingress with a backend but no rule
					defLoc.Backend = backendUpstream.Name
					defLoc.Service = backendUpstream.Service
					defLoc.Ingress = ing
					// TODO: Redirect and rewrite can affect the catch all behavior, skip for now
					originalRedirect := defLoc.Redirect
					originalRewrite := defLoc.Rewrite
					locationApplyAnnotations(defLoc, anns)
					defLoc.Redirect = originalRedirect
					defLoc.Rewrite = originalRewrite
				} else {
					log.Info().Msgf("Ingress %q defines both a backend and rules. Using its backend as default upstream for all its rules.", ingKey)
				}
			}
		}

		for _, rule := range ing.Spec.Rules {
			host := rule.Host
			if host == "" {
				host = defServerName
			}

			if _, ok := servers[host]; ok {
				// server already configured
				continue
			}

			loc := &Location{
				Path:         rootLocation,
				PathType:     &pathTypePrefix,
				IsDefBackend: true,
				Backend:      un,
				Ingress:      ing,
				Service:      &corev1.Service{},
			}
			locationApplyAnnotations(loc, anns)

			servers[host] = &Server{
				Hostname: host,
				Locations: []*Location{
					loc,
				},
				SSLPassthrough:         anns.SSLPassthrough,
				SSLCiphers:             anns.SSLCipher.SSLCiphers,
				SSLPreferServerCiphers: anns.SSLCipher.SSLPreferServerCiphers,
			}
		}
	}

	// configure default location, alias, and SSL
	for _, ing := range data {
		ingKey := k8s.MetaNamespaceKey(ing)
		anns := toAnnotations(ing.ParsedAnnotations)

		if !p.AllowSnippetAnnotations {
			dropSnippetDirectives(anns, ingKey)
		}

		if anns.Canary.Enabled {
			log.Info().Msgf("Ingress %v is marked as Canary, ignoring", ingKey)
			continue
		}

		for _, rule := range ing.Spec.Rules {
			host := rule.Host
			if host == "" {
				host = defServerName
			}

			if len(servers[host].Aliases) == 0 {
				servers[host].Aliases = anns.Aliases
				if aliases := allAliases[host]; len(aliases) == 0 {
					allAliases[host] = anns.Aliases
				}
			} else {
				log.Warn().Msgf("Aliases already configured for server %q, skipping (Ingress %q)", host, ingKey)
			}

			if anns.ServerSnippet != "" {
				if servers[host].ServerSnippet == "" {
					servers[host].ServerSnippet = anns.ServerSnippet
				} else {
					log.Warn().Msgf("Server snippet already configured for server %q, skipping (Ingress %q)",
						host, ingKey)
				}
			}

			if !servers[host].SSLPassthrough && anns.SSLPassthrough {
				servers[host].SSLPassthrough = true
			}

			// only add SSL ciphers if the server does not have them previously configured
			if servers[host].SSLCiphers == "" && anns.SSLCipher.SSLCiphers != "" {
				servers[host].SSLCiphers = anns.SSLCipher.SSLCiphers
			}

			// only add SSLPreferServerCiphers if the server does not have them previously configured
			if servers[host].SSLPreferServerCiphers == "" && anns.SSLCipher.SSLPreferServerCiphers != "" {
				servers[host].SSLPreferServerCiphers = anns.SSLCipher.SSLPreferServerCiphers
			}

			// only add a certificate if the server does not have one previously configured
			if servers[host].SSLCert != nil {
				continue
			}

			if len(ing.Spec.TLS) == 0 {
				log.Info().Msgf("Ingress %q does not contains a TLS section.", ingKey)
				continue
			}

			tlsSecretName := extractTLSSecretName(host, ing, p.GetLocalSSLCert)
			if tlsSecretName == "" {
				log.Info().Msgf("Host %q is listed in the TLS section but secretName is empty. Using default certificate", host)
				// servers[host].SSLCert = n.getDefaultSSLCertificate() // FIXME
				continue
			}

			secrKey := fmt.Sprintf("%v/%v", ing.Namespace, tlsSecretName)
			cert, err := p.GetLocalSSLCert(secrKey)
			if err != nil {
				log.Warn().Msgf("Error getting SSL certificate %q: %v. Using default certificate", secrKey, err)
				servers[host].SSLCert = p.getDefaultSSLCertificate()
				continue
			}

			if cert.Certificate == nil {
				log.Warn().Msgf("SSL certificate %q does not contain a valid SSL certificate for server %q", secrKey, host)
				log.Warn().Msgf("Using default certificate")
				servers[host].SSLCert = p.getDefaultSSLCertificate()
				continue
			}

			err = cert.Certificate.VerifyHostname(host)
			if err != nil {
				log.Warn().Msgf("Unexpected error validating SSL certificate %q for server %q: %v", secrKey, host, err)
				log.Warn().Msgf("Validating certificate against DNS names. This will be deprecated in a future version")
				// check the Common Name field
				// https://github.com/golang/go/issues/22922
				err := verifyHostname(host, cert.Certificate)
				if err != nil {
					log.Warn().Msgf("SSL certificate %q does not contain a Common Name or Subject Alternative Name for server %q: %v", secrKey, host, err)
					log.Warn().Msgf("Using default certificate")
					servers[host].SSLCert = p.getDefaultSSLCertificate()
					continue
				}
			}

			servers[host].SSLCert = cert

			now := time.Now()
			if cert.ExpireTime.Before(now) {
				log.Warn().Msgf("SSL certificate for server %q expired (%v)", host, cert.ExpireTime)
			} else if cert.ExpireTime.Before(now.Add(240 * time.Hour)) {
				log.Warn().Msgf("SSL certificate for server %q is about to expire (%v)", host, cert.ExpireTime)
			}
		}
	}

	for host, hostAliases := range allAliases {
		if _, ok := servers[host]; !ok {
			continue
		}

		uniqAliases := sets.NewString()
		for _, alias := range hostAliases {
			if alias == host {
				continue
			}

			if _, ok := servers[alias]; ok {
				continue
			}

			if uniqAliases.Has(alias) {
				continue
			}

			uniqAliases.Insert(alias)
		}

		servers[host].Aliases = uniqAliases.List()
	}

	return servers
}

// TODO
func (p *Provider) GetLocalSSLCert(name string) (*SSLCert, error) {
	// TODO: implement or FIXME

	return nil, nil
}

// TODO
func (n *Provider) getDefaultSSLCertificate() *SSLCert {

	// TODO: FIXME

	// // read custom default SSL certificate, fall back to generated default certificate
	// if n.cfg.DefaultSSLCertificate != "" {
	// 	certificate, err := n.store.GetLocalSSLCert(n.cfg.DefaultSSLCertificate)
	// 	if err == nil {
	// 		return certificate
	// 	}

	// log.Warn().Msgf("Error loading custom default certificate, falling back to generated default:\n%v", err)
	// }

	// return n.cfg.FakeCertificate
	return nil
}

// extractTLSSecretName returns the name of the Secret containing a SSL
// certificate for the given host name, or an empty string.
func extractTLSSecretName(host string, ing *Ingress,
	getLocalSSLCert func(string) (*SSLCert, error),
) string {
	if ing == nil {
		return ""
	}

	// naively return Secret name from TLS spec if host name matches
	lowercaseHost := toLowerCaseASCII(host)
	for _, tls := range ing.Spec.TLS {
		for _, tlsHost := range tls.Hosts {
			if toLowerCaseASCII(tlsHost) == lowercaseHost {
				return tls.SecretName
			}
		}
	}

	// no TLS host matching host name, try each TLS host for matching SAN or CN
	for _, tls := range ing.Spec.TLS {
		if tls.SecretName == "" {
			// There's no secretName specified, so it will never be available
			continue
		}

		secrKey := fmt.Sprintf("%v/%v", ing.Namespace, tls.SecretName)

		cert, err := getLocalSSLCert(secrKey)
		if err != nil {
			log.Warn().Msgf("Error getting SSL certificate %q: %v", secrKey, err)
			continue
		}

		if cert == nil || cert.Certificate == nil {
			continue
		}

		err = cert.Certificate.VerifyHostname(host)
		if err != nil {
			continue
		}
		log.Info().Msgf("Found SSL certificate matching host %q: %q", host, secrKey)
		return tls.SecretName
	}

	return ""
}

// getServiceClusterEndpoint returns an Endpoint corresponding to the ClusterIP
// field of a Service.
func (p *Provider) getServiceClusterEndpoint(svcKey string, backend *netv1.IngressBackend) (endpoint Endpoint, err error) {
	svc, err := p.GetService(svcKey)
	if err != nil {
		return endpoint, fmt.Errorf("service %q does not exist", svcKey)
	}

	if svc.Spec.ClusterIP == "" || svc.Spec.ClusterIP == "None" {
		return endpoint, fmt.Errorf("no ClusterIP found for Service %q", svcKey)
	}

	endpoint.Address = svc.Spec.ClusterIP

	// if the Service port is referenced by name in the Ingress, lookup the
	// actual port in the service spec
	if backend.Service != nil {
		_, svcportintorstr := upstreamServiceNameAndPort(backend.Service)
		if svcportintorstr.Type == intstr.String {
			var port int32 = -1
			for _, svcPort := range svc.Spec.Ports {
				if svcPort.Name == svcportintorstr.String() {
					port = svcPort.Port
					break
				}
			}
			if port == -1 {
				return endpoint, fmt.Errorf("service %q does not have a port named %q", svc.Name, svcportintorstr.String())
			}
			endpoint.Port = fmt.Sprintf("%d", port)
		} else {
			endpoint.Port = svcportintorstr.String()
		}
	}

	return endpoint, err
}

// getDefaultUpstream returns the upstream associated with the default backend.
// Configures the upstream to return HTTP code 503 in case of error.
func (p *Provider) getDefaultUpstream() *Backend {
	upstream := &Backend{
		Name: defUpstreamName,
	}
	svcKey := p.DefaultBackendService

	if svcKey == "" {
		upstream.Endpoints = append(upstream.Endpoints, p.DefaultEndpoint())
		return upstream
	}

	svc, err := p.GetService(svcKey)
	if err != nil {
		log.Warn().Msgf("Error getting default backend %q: %v", svcKey, err)
		upstream.Endpoints = append(upstream.Endpoints, p.DefaultEndpoint())
		return upstream
	}
	var zone string
	if p.EnableTopologyAwareRouting {
		zone = getIngressPodZone(svc)
	} else {
		zone = emptyZone
	}
	endps := getEndpointsFromSlices(svc, &svc.Spec.Ports[0], corev1.ProtocolTCP, zone, p.GetServiceEndpointsSlices)
	if len(endps) == 0 {
		log.Warn().Msgf("Service %q does not have any active Endpoint", svcKey)
		endps = []Endpoint{p.DefaultEndpoint()}
	}

	upstream.Service = svc
	upstream.Endpoints = append(upstream.Endpoints, endps...)
	return upstream
}

// TODO
func (p *Provider) getBackendServers(ingresses []*Ingress) ([]*Backend, []*Server) {
	du := p.getDefaultUpstream()
	upstreams := p.createUpstreams(ingresses, du)
	servers := p.createServers(ingresses, upstreams, du)
	// var upstreams map[string]*Backend
	// var servers map[string]*Server

	var canaryIngresses []*Ingress

	for _, ing := range ingresses {
		ingKey := k8s.MetaNamespaceKey(ing)
		anns := ing.ParsedAnnotationsNGINX

		if !p.AllowSnippetAnnotations {
			dropSnippetDirectives(anns, ingKey)
		}

		for _, rule := range ing.Spec.Rules {
			host := rule.Host
			if host == "" {
				host = defServerName
			}

			server := servers[host]
			if server == nil {
				server = servers[defServerName]
			}

			if rule.HTTP == nil &&
				host != defServerName {
				log.Info().Msgf("Ingress %q does not contain any HTTP rule, using default backend", ingKey)
				continue
			}

			if server.AuthTLSError == "" && anns.CertificateAuth.AuthTLSError != "" {
				server.AuthTLSError = anns.CertificateAuth.AuthTLSError
			}

			if server.CertificateAuth.CAFileName == "" {
				server.CertificateAuth = anns.CertificateAuth
				if server.CertificateAuth.Secret != "" && server.CertificateAuth.CAFileName == "" {
					log.Info().Msgf("Secret %q has no 'ca.crt' key, mutual authentication disabled for Ingress %q",
						server.CertificateAuth.Secret, ingKey)
				}
			} else {
				log.Info().Msgf("Server %q is already configured for mutual authentication (Ingress %q)",
					server.Hostname, ingKey)
			}

			if !p.ProxySSLLocationOnly {
				if server.ProxySSL.CAFileName == "" {
					server.ProxySSL = anns.ProxySSL
					if server.ProxySSL.Secret != "" && server.ProxySSL.CAFileName == "" {
						log.Info().Msgf("Secret %q has no 'ca.crt' key, client cert authentication disabled for Ingress %q",
							server.ProxySSL.Secret, ingKey)
					}
				} else {
					log.Info().Msgf("Server %q is already configured for client cert authentication (Ingress %q)",
						server.Hostname, ingKey)
				}
			}

			if rule.HTTP == nil {
				log.Info().Msgf("Ingress %q does not contain any HTTP rule, using default backend", ingKey)
				continue
			}

			for _, path := range rule.HTTP.Paths {
				if path.Backend.Service == nil {
					// skip non-service backends
					log.Info().Msgf("Ingress %q and path %q does not contain a service backend, using default backend", ingKey, path.Path)
					continue
				}

				upsName := upstreamName(ing.Namespace, path.Backend.Service)

				ups := upstreams[upsName]

				// Backend is not referenced to by a server
				if ups.NoServer {
					continue
				}

				nginxPath := rootLocation
				if path.Path != "" {
					nginxPath = path.Path
				}

				addLoc := true
				for _, loc := range server.Locations {
					if loc.Path != nginxPath {
						continue
					}

					// Same paths but different types are allowed
					// (same type means overlap in the path definition)
					if !apiequality.Semantic.DeepEqual(loc.PathType, path.PathType) {
						break
					}

					addLoc = false

					if !loc.IsDefBackend {
						log.Info().Msgf("Location %q already configured for server %q with upstream %q (Ingress %q)",
							loc.Path, server.Hostname, loc.Backend, ingKey)
						break
					}

					log.Info().Msgf("Replacing location %q for server %q with upstream %q to use upstream %q (Ingress %q)",
						loc.Path, server.Hostname, loc.Backend, ups.Name, ingKey)

					loc.Backend = ups.Name
					loc.IsDefBackend = false
					loc.Port = ups.Port
					loc.Service = ups.Service
					loc.Ingress = ing

					locationApplyAnnotations(loc, anns)

					if loc.Redirect.FromToWWW {
						server.RedirectFromToWWW = true
					}

					break
				}

				// new location
				if addLoc {
					log.Info().Msgf("Adding location %q for server %q with upstream %q (Ingress %q)",
						nginxPath, server.Hostname, ups.Name, ingKey)
					loc := &Location{
						Path:         nginxPath,
						PathType:     path.PathType,
						Backend:      ups.Name,
						IsDefBackend: false,
						Service:      ups.Service,
						Port:         ups.Port,
						Ingress:      ing,
					}
					locationApplyAnnotations(loc, anns)

					if loc.Redirect.FromToWWW {
						server.RedirectFromToWWW = true
					}
					server.Locations = append(server.Locations, loc)
				}

				if ups.SessionAffinity.AffinityType == "" {
					ups.SessionAffinity.AffinityType = anns.SessionAffinity.Type
				}

				if ups.SessionAffinity.AffinityMode == "" {
					ups.SessionAffinity.AffinityMode = anns.SessionAffinity.Mode
				}

				if anns.SessionAffinity.Type == "cookie" {
					cookiePath := anns.SessionAffinity.Cookie.Path
					if anns.Rewrite.UseRegex && cookiePath == "" {
						log.Warn().Msgf("session-cookie-path should be set when use-regex is true")
					}

					ups.SessionAffinity.CookieSessionAffinity.Name = anns.SessionAffinity.Cookie.Name
					ups.SessionAffinity.CookieSessionAffinity.Expires = anns.SessionAffinity.Cookie.Expires
					ups.SessionAffinity.CookieSessionAffinity.MaxAge = anns.SessionAffinity.Cookie.MaxAge
					ups.SessionAffinity.CookieSessionAffinity.Secure = anns.SessionAffinity.Cookie.Secure
					ups.SessionAffinity.CookieSessionAffinity.Path = cookiePath
					ups.SessionAffinity.CookieSessionAffinity.Domain = anns.SessionAffinity.Cookie.Domain
					ups.SessionAffinity.CookieSessionAffinity.SameSite = anns.SessionAffinity.Cookie.SameSite
					ups.SessionAffinity.CookieSessionAffinity.ConditionalSameSiteNone = anns.SessionAffinity.Cookie.ConditionalSameSiteNone
					ups.SessionAffinity.CookieSessionAffinity.ChangeOnFailure = anns.SessionAffinity.Cookie.ChangeOnFailure

					locs := ups.SessionAffinity.CookieSessionAffinity.Locations
					if _, ok := locs[host]; !ok {
						locs[host] = []string{}
					}
					locs[host] = append(locs[host], path.Path)

					if len(server.Aliases) > 0 {
						for _, alias := range server.Aliases {
							if _, ok := locs[alias]; !ok {
								locs[alias] = []string{}
							}
							locs[alias] = append(locs[alias], path.Path)
						}
					}
				}
			}
		}

		// set aside canary ingresses to merge later
		if anns.Canary.Enabled {
			canaryIngresses = append(canaryIngresses, ing)
		}
	}

	if nonCanaryIngressExists(ingresses, canaryIngresses) {
		for _, canaryIng := range canaryIngresses {
			mergeAlternativeBackends(canaryIng, upstreams, servers)
		}
	}

	aUpstreams := make([]*Backend, 0, len(upstreams))

	for _, upstream := range upstreams {
		aUpstreams = append(aUpstreams, upstream)

		if upstream.Name == defUpstreamName {
			continue
		}

		isHTTPSfrom := []*Server{}
		for _, server := range servers {
			for _, location := range server.Locations {
				// use default backend
				if !shouldCreateUpstreamForLocationDefaultBackend(upstream, location) {
					continue
				}

				if len(location.DefaultBackend.Spec.Ports) == 0 {
					log.Error().Msgf("Custom default backend service %v/%v has no ports. Ignoring", location.DefaultBackend.Namespace, location.DefaultBackend.Name)
					continue
				}

				sp := location.DefaultBackend.Spec.Ports[0]
				var zone string
				if p.EnableTopologyAwareRouting {
					zone = getIngressPodZone(location.DefaultBackend)
				} else {
					zone = emptyZone
				}
				endps := getEndpointsFromSlices(location.DefaultBackend, &sp, corev1.ProtocolTCP, zone, p.GetServiceEndpointsSlices)
				// custom backend is valid only if contains at least one endpoint
				if len(endps) > 0 {
					name := fmt.Sprintf("custom-default-backend-%v-%v", location.DefaultBackend.GetNamespace(), location.DefaultBackend.GetName())
					log.Info().Msgf("Creating \"%v\" upstream based on default backend annotation", name)

					nb := upstream.DeepCopy()
					nb.Name = name
					nb.Endpoints = endps
					aUpstreams = append(aUpstreams, nb)
					location.DefaultBackendUpstreamName = name

					if len(upstream.Endpoints) == 0 {
						log.Info().Msgf("Upstream %q has no active Endpoint, so using custom default backend for location %q in server %q (Service \"%v/%v\")",
							upstream.Name, location.Path, server.Hostname, location.DefaultBackend.Namespace, location.DefaultBackend.Name)

						location.Backend = name
					}
				}

				if server.SSLPassthrough {
					if location.Path == rootLocation {
						if location.Backend == defUpstreamName {
							log.Warn().Msgf("Server %q has no default backend, ignoring SSL Passthrough.", server.Hostname)
							continue
						}
						isHTTPSfrom = append(isHTTPSfrom, server)
					}
				}
			}
		}

		if len(isHTTPSfrom) > 0 {
			upstream.SSLPassthrough = true
		}
	}

	aServers := make([]*Server, 0, len(servers))
	for _, value := range servers {
		sort.SliceStable(value.Locations, func(i, j int) bool {
			return value.Locations[i].Path > value.Locations[j].Path
		})

		sort.SliceStable(value.Locations, func(i, j int) bool {
			return len(value.Locations[i].Path) > len(value.Locations[j].Path)
		})
		aServers = append(aServers, value)
	}

	sort.SliceStable(aUpstreams, func(a, b int) bool {
		return aUpstreams[a].Name < aUpstreams[b].Name
	})

	sort.SliceStable(aServers, func(i, j int) bool {
		return aServers[i].Hostname < aServers[j].Hostname
	})

	return aUpstreams, aServers
}

// checks conditions for whether or not an upstream should be created for a custom default backend
func shouldCreateUpstreamForLocationDefaultBackend(upstream *Backend, location *Location) bool {
	return (upstream.Name == location.Backend) &&
		(len(upstream.Endpoints) == 0 || len(location.CustomHTTPErrors) != 0) &&
		location.DefaultBackend != nil
}

func locationApplyAnnotations(loc *Location, anns *ParsedAnnotationsNGINX) {
	loc.BasicDigestAuth = anns.BasicDigestAuth
	loc.ClientBodyBufferSize = anns.ClientBodyBufferSize
	loc.CustomHeaders = anns.CustomHeaders
	loc.ConfigurationSnippet = anns.ConfigurationSnippet
	loc.CorsConfig = anns.CorsConfig
	loc.ExternalAuth = anns.ExternalAuth
	loc.EnableGlobalAuth = anns.EnableGlobalAuth
	loc.HTTP2PushPreload = anns.HTTP2PushPreload
	loc.Opentelemetry = anns.Opentelemetry
	loc.Proxy = anns.Proxy
	loc.ProxySSL = anns.ProxySSL
	loc.RateLimit = anns.RateLimit
	loc.Redirect = anns.Redirect
	loc.Rewrite = anns.Rewrite
	loc.UpstreamVhost = anns.UpstreamVhost
	loc.Denylist = anns.Denylist
	loc.Allowlist = anns.Allowlist
	loc.Denied = anns.Denied
	loc.XForwardedPrefix = anns.XForwardedPrefix
	loc.UsePortInRedirects = anns.UsePortInRedirects
	loc.Connection = anns.Connection
	loc.Logs = anns.Logs
	loc.DefaultBackend = anns.DefaultBackend
	loc.BackendProtocol = anns.BackendProtocol
	loc.FastCGI = anns.FastCGI
	loc.CustomHTTPErrors = anns.CustomHTTPErrors
	loc.DisableProxyInterceptErrors = anns.DisableProxyInterceptErrors
	loc.ModSecurity = anns.ModSecurity
	loc.Satisfy = anns.Satisfy
	loc.Mirror = anns.Mirror

	loc.DefaultBackendUpstreamName = defUpstreamName
}

// newUpstream creates an upstream without servers.
func newUpstream(name string) *Backend {
	return &Backend{
		Name:      name,
		Endpoints: []Endpoint{},
		Service:   &corev1.Service{},
		SessionAffinity: SessionAffinityConfig{
			CookieSessionAffinity: CookieSessionAffinity{
				Locations: make(map[string][]string),
			},
		},
	}
}

// upstreamName returns a formatted upstream name based on namespace, service, and port
func upstreamName(namespace string, service *netv1.IngressServiceBackend) string {
	if service != nil {
		if service.Port.Number > 0 {
			return fmt.Sprintf("%s-%s-%d", namespace, service.Name, service.Port.Number)
		}
		if service.Port.Name != "" {
			return fmt.Sprintf("%s-%s-%s", namespace, service.Name, service.Port.Name)
		}
	}
	return fmt.Sprintf("%s-INVALID", namespace)
}

// upstreamServiceNameAndPort verifies if service is not nil, and then return the
// correct serviceName and Port
func upstreamServiceNameAndPort(service *netv1.IngressServiceBackend) (string, intstr.IntOrString) {
	if service != nil {
		if service.Port.Number > 0 {
			return service.Name, intstr.FromInt(int(service.Port.Number))
		}
		if service.Port.Name != "" {
			return service.Name, intstr.FromString(service.Port.Name)
		}
	}
	return "", intstr.IntOrString{}
}

func dropSnippetDirectives(anns *ParsedAnnotationsNGINX, ingKey string) {
	if anns != nil {
		if anns.ConfigurationSnippet != "" {
			log.Info().Msgf("Ingress %q tried to use configuration-snippet and the annotation is disabled by the admin. Removing the annotation", ingKey)
			anns.ConfigurationSnippet = ""
		}
		if anns.ServerSnippet != "" {
			log.Info().Msgf("Ingress %q tried to use server-snippet and the annotation is disabled by the admin. Removing the annotation", ingKey)
			anns.ServerSnippet = ""
		}

		if anns.ModSecurity.Snippet != "" {
			log.Info().Msgf("Ingress %q tried to use modsecurity-snippet and the annotation is disabled by the admin. Removing the annotation", ingKey)
			anns.ModSecurity.Snippet = ""
		}

		if anns.ExternalAuth.AuthSnippet != "" {
			log.Info().Msgf("Ingress %q tried to use auth-snippet and the annotation is disabled by the admin. Removing the annotation", ingKey)
			anns.ExternalAuth.AuthSnippet = ""
		}

		if anns.StreamSnippet != "" {
			log.Info().Msgf("Ingress %q tried to use stream-snippet and the annotation is disabled by the admin. Removing the annotation", ingKey)
			anns.StreamSnippet = ""
		}
	}
}

// OK to merge canary ingresses iff there exists one or more ingresses to potentially merge into
func nonCanaryIngressExists(ingresses, canaryIngresses []*Ingress) bool {
	return len(ingresses)-len(canaryIngresses) > 0
}

// ensure that the following conditions are met
// 1) names of backends do not match and canary doesn't merge into itself
// 2) primary name is not the default upstream
// 3) the primary has a server
func canMergeBackend(primary, alternative *Backend) bool {
	return alternative != nil && primary.Name != alternative.Name && primary.Name != defUpstreamName && !primary.NoServer
}

// Performs the merge action and checks to ensure that one two alternative backends do not merge into each other
func mergeAlternativeBackend(ing *Ingress, priUps, altUps *Backend) bool {
	if priUps.NoServer {
		log.Warn().Msgf("unable to merge alternative backend %v into primary backend %v because %v is a primary backend",
			altUps.Name, priUps.Name, priUps.Name)
		return false
	}

	for _, ab := range priUps.AlternativeBackends {
		if ab == altUps.Name {
			log.Info().Msgf("skip merge alternative backend %v into %v, it's already present", altUps.Name, priUps.Name)
			return true
		}
	}

	if ing.ParsedAnnotationsNGINX != nil && ing.ParsedAnnotationsNGINX.SessionAffinity.CanaryBehavior != "legacy" {
		priUps.SessionAffinity.DeepCopyInto(&altUps.SessionAffinity)
	}

	priUps.AlternativeBackends = append(priUps.AlternativeBackends, altUps.Name)

	return true
}

// Compares an Ingress of a potential alternative backend's rules with each existing server and finds matching host + path pairs.
// If a match is found, we know that this server should back the alternative backend and add the alternative backend
// to a backend's alternative list.
// If no match is found, then the serverless backend is deleted.
func mergeAlternativeBackends(ing *Ingress, upstreams map[string]*Backend,
	servers map[string]*Server,
) {
	// merge catch-all alternative backends
	if ing.Spec.DefaultBackend != nil {
		upsName := upstreamName(ing.Namespace, ing.Spec.DefaultBackend.Service)

		altUps := upstreams[upsName]

		if altUps == nil {
			log.Warn().Msgf("alternative backend %s has already been removed", upsName)
		} else {
			merged := false
			altEqualsPri := false

			for _, loc := range servers[defServerName].Locations {
				priUps, ok := upstreams[loc.Backend]
				if !ok {
					log.Warn().Msgf("cannot find primary backend %s for location %s%s", loc.Backend, servers[defServerName].Hostname, loc.Path)
					continue
				}
				altEqualsPri = altUps.Name == priUps.Name
				if altEqualsPri {
					log.Warn().Msgf("alternative upstream %s in Ingress %s/%s is primary upstream in Other Ingress for location %s%s!",
						altUps.Name, ing.Namespace, ing.Name, servers[defServerName].Hostname, loc.Path)
					break
				}

				if canMergeBackend(priUps, altUps) {
					log.Info().Msgf("matching backend %v found for alternative backend %v",
						priUps.Name, altUps.Name)

					merged = mergeAlternativeBackend(ing, priUps, altUps)
				}
			}

			if !altEqualsPri && !merged {
				log.Warn().Msgf("unable to find real backend for alternative backend %v. Deleting.", altUps.Name)
				delete(upstreams, altUps.Name)
			}
		}
	}

	for _, rule := range ing.Spec.Rules {
		host := rule.Host
		if host == "" {
			host = defServerName
		}

		for _, path := range rule.HTTP.Paths {
			if path.Backend.Service == nil {
				// skip non-service backends
				log.Info().Msgf("Ingress %q and path %q does not contain a service backend, using default backend", k8s.MetaNamespaceKey(ing), path.Path)
				continue
			}

			upsName := upstreamName(ing.Namespace, path.Backend.Service)

			altUps := upstreams[upsName]

			if altUps == nil {
				log.Warn().Msgf("alternative backend %s has already been removed", upsName)
				continue
			}

			merged := false
			altEqualsPri := false

			server, ok := servers[host]
			if !ok {
				log.Error().Msgf("cannot merge alternative backend %s into hostname %s that does not exist",
					altUps.Name,
					host)

				continue
			}

			// find matching paths
			for _, loc := range server.Locations {
				priUps, ok := upstreams[loc.Backend]
				if !ok {
					log.Warn().Msgf("cannot find primary backend %s for location %s%s", loc.Backend, server.Hostname, loc.Path)
					continue
				}
				altEqualsPri = altUps.Name == priUps.Name
				if altEqualsPri {
					log.Warn().Msgf("alternative upstream %s in Ingress %s/%s is primary upstream in Other Ingress for location %s%s!",
						altUps.Name, ing.Namespace, ing.Name, server.Hostname, loc.Path)
					break
				}

				if canMergeBackend(priUps, altUps) && loc.Path == path.Path && *loc.PathType == *path.PathType {
					log.Info().Msgf("matching backend %v found for alternative backend %v",
						priUps.Name, altUps.Name)

					merged = mergeAlternativeBackend(ing, priUps, altUps)
				}
			}

			if !altEqualsPri && !merged {
				log.Warn().Msgf("unable to find real backend for alternative backend %v. Deleting.", altUps.Name)
				delete(upstreams, altUps.Name)
			}
		}
	}
}

// updateServerLocations inspects the generated locations configuration for a server
// normalizing the path and adding an additional exact location when is possible
func updateServerLocations(locations []*Location) []*Location {
	newLocations := []*Location{}

	// get Exact locations to check if one already exists
	exactLocations := map[string]*Location{}
	for _, location := range locations {
		if *location.PathType == pathTypeExact {
			exactLocations[location.Path] = location
		}
	}

	for _, location := range locations {
		// location / does not require any update
		if location.Path == rootLocation {
			newLocations = append(newLocations, location)
			continue
		}

		location.IngressPath = location.Path

		// only Prefix locations could require an additional location block
		if *location.PathType != pathTypePrefix {
			newLocations = append(newLocations, location)
			continue
		}

		// locations with rewrite or using regular expressions are not modified
		if needsRewrite(location) || location.Rewrite.UseRegex {
			newLocations = append(newLocations, location)
			continue
		}

		// If exists an Exact location is not possible to create a new one.
		if _, alreadyExists := exactLocations[location.Path]; alreadyExists {
			// normalize path. Must end in /
			location.Path = normalizePrefixPath(location.Path)
			newLocations = append(newLocations, location)
			continue
		}

		var el Location = *location

		// normalize path. Must end in /
		location.Path = normalizePrefixPath(location.Path)
		newLocations = append(newLocations, location)

		// add exact location
		exactLocation := &el
		exactLocation.PathType = &pathTypeExact

		newLocations = append(newLocations, exactLocation)
	}

	return newLocations
}

func normalizePrefixPath(path string) string {
	if path == rootLocation {
		return rootLocation
	}

	if !strings.HasSuffix(path, "/") {
		return fmt.Sprintf("%v/", path)
	}

	return path
}

func needsRewrite(location *Location) bool {
	if location.Rewrite.Target != "" && location.Rewrite.Target != location.Path {
		return true
	}

	return false
}

// newTrafficShapingPolicy creates new TrafficShapingPolicy instance using canary configuration
func newTrafficShapingPolicy(cfg *canary.Config) TrafficShapingPolicy {
	return TrafficShapingPolicy{
		Weight:        cfg.Weight,
		WeightTotal:   cfg.WeightTotal,
		Header:        cfg.Header,
		HeaderValue:   cfg.HeaderValue,
		HeaderPattern: cfg.HeaderPattern,
		Cookie:        cfg.Cookie,
	}
}

func getIngressPodZone(svc *corev1.Service) string {
	svcKey := k8s.MetaNamespaceKey(svc)
	if svcZoneAnnotation, ok := svc.ObjectMeta.GetAnnotations()[corev1.AnnotationTopologyMode]; ok {
		if strings.EqualFold(svcZoneAnnotation, "auto") {
			if foundZone, ok := k8s.IngressNodeDetails.GetLabels()[corev1.LabelTopologyZone]; ok {
				log.Info().Msgf("Svc has topology aware annotation enabled, try to use zone %q where controller pod is running for Service %q ", foundZone, svcKey)
				return foundZone
			}
		}
	}
	if svc.Spec.TrafficDistribution != nil && *svc.Spec.TrafficDistribution == corev1.ServiceTrafficDistributionPreferClose {
		if foundZone, ok := k8s.IngressNodeDetails.GetLabels()[corev1.LabelTopologyZone]; ok {
			log.Info().Msgf("Svc has traffic distribution enabled, try to use zone %q where controller pod is running for Service %q ", foundZone, svcKey)
			return foundZone
		}
	}

	return emptyZone
}

func externalNamePorts(name string, svc *corev1.Service) *corev1.ServicePort {
	port, err := strconv.Atoi(name) // #nosec
	if err != nil {
		// not a number. check port names.
		for _, svcPort := range svc.Spec.Ports {
			if svcPort.Name != name {
				continue
			}

			tp := svcPort.TargetPort
			if tp.IntValue() == 0 {
				tp = intstr.FromInt(int(svcPort.Port))
			}

			return &corev1.ServicePort{
				Protocol:   "TCP",
				Port:       svcPort.Port,
				TargetPort: tp,
			}
		}
	}

	for _, svcPort := range svc.Spec.Ports {
		//nolint:gosec // Ignore G109 error
		if svcPort.Port != int32(port) {
			continue
		}

		tp := svcPort.TargetPort
		if tp.IntValue() == 0 {
			tp = intstr.FromInt(port)
		}

		return &corev1.ServicePort{
			Protocol:   "TCP",
			Port:       svcPort.Port,
			TargetPort: svcPort.TargetPort,
		}
	}

	// ExternalName without port
	return &corev1.ServicePort{
		Protocol: "TCP",
		//nolint:gosec // Ignore G109 error
		Port:       int32(port),
		TargetPort: intstr.FromInt(port),
	}
}

func (p *Provider) GetServiceEndpointsSlices(svcKey string) ([]*discoveryv1.EndpointSlice, error) {
	res := strings.Split(svcKey, "/")
	return p.k8sClient.GetEndpointSlicesForService(res[0], res[1])
}

// getEndpointsFromSlices returns a list of Endpoint structs for a given service/target port combination.
func getEndpointsFromSlices(s *corev1.Service, port *corev1.ServicePort, proto corev1.Protocol, zoneForHints string,
	getServiceEndpointsSlices func(string) ([]*discoveryv1.EndpointSlice, error),
) []Endpoint {
	upsServers := []Endpoint{}

	if s == nil || port == nil {
		return upsServers
	}

	// we need to check if there is at least one endpoint with controller zone
	// if we use traffic distribution
	useTrafficDistribution := s.Spec.TrafficDistribution != nil && *s.Spec.TrafficDistribution == corev1.ServiceTrafficDistributionPreferClose

	// using a map avoids duplicated upstream servers when the service
	// contains multiple port definitions sharing the same targetport
	processedUpstreamServers := make(map[string]struct{})

	svcKey := k8s.MetaNamespaceKey(s)
	var useTopologyHints bool

	// ExternalName services
	if s.Spec.Type == corev1.ServiceTypeExternalName {
		if ip := net.ParseIP(s.Spec.ExternalName); s.Spec.ExternalName == "localhost" ||
			(ip != nil && ip.IsLoopback()) {
			log.Error().Msgf("Invalid attempt to use localhost name %s in %q", s.Spec.ExternalName, svcKey)
			return upsServers
		}

		log.Info().Msgf("Ingress using Service %q of type ExternalName.", svcKey)
		targetPort := port.TargetPort.IntValue()
		// if the externalName is not an IP address we need to validate is a valid FQDN
		if net.ParseIP(s.Spec.ExternalName) == nil {
			externalName := strings.TrimSuffix(s.Spec.ExternalName, ".")
			if errs := validation.IsDNS1123Subdomain(externalName); len(errs) > 0 {
				log.Error().Msgf("Invalid DNS name %s: %v", s.Spec.ExternalName, errs)
				return upsServers
			}
		}

		return append(upsServers, Endpoint{
			Address: s.Spec.ExternalName,
			Port:    fmt.Sprintf("%v", targetPort),
		})
	}

	log.Info().Msgf("Getting Endpoints from endpointSlices for Service %q and port %v", svcKey, port.String())
	epss, err := getServiceEndpointsSlices(svcKey)
	if err != nil {
		log.Warn().Msgf("Error obtaining Endpoints for Service %q: %v", svcKey, err)
		return upsServers
	}
	// loop over all endpointSlices generated for service
	for _, eps := range epss {
		var ports []int32
		if len(eps.Ports) == 0 && port.TargetPort.Type == intstr.Int {
			// When ports is empty, it indicates that there are no defined ports, using svc targePort if it's a number
			log.Info().Msgf("No ports found on endpointSlice, using service TargetPort %v for Service %q", port.String(), svcKey)
			ports = append(ports, port.TargetPort.IntVal)
		} else {
			for _, epPort := range eps.Ports {
				if !reflect.DeepEqual(*epPort.Protocol, proto) {
					continue
				}
				var targetPort int32
				if port.Name == "" {
					// port.Name is optional if there is only one port
					targetPort = *epPort.Port
				} else if port.Name == *epPort.Name {
					targetPort = *epPort.Port
				}
				if targetPort == 0 && port.TargetPort.Type == intstr.Int {
					// use service target port if it's a number and no port name matched
					// https://github.com/kubernetes/ingress-nginx/issues/7390
					targetPort = port.TargetPort.IntVal
				}
				if targetPort == 0 {
					continue
				}
				ports = append(ports, targetPort)
			}
		}
		useTopologyHints = false
		if zoneForHints != emptyZone {
			useTopologyHints = true

			// check if endpointslices have zone hints with controller zone
			if useTrafficDistribution {
				foundEndpointsForZone := false
				for _, ep := range eps.Endpoints {
					if ep.Hints == nil {
						continue
					}
					for _, epzone := range ep.Hints.ForZones {
						if epzone.Name == zoneForHints {
							foundEndpointsForZone = true
							break
						}
					}
				}
				if !foundEndpointsForZone {
					log.Info().Msgf("No endpoints found for zone %q in Service %q", zoneForHints, svcKey)
					useTopologyHints = false
				}
			}

			// check if all endpointslices have zone hints
			for _, ep := range eps.Endpoints {
				if ep.Hints == nil || len(ep.Hints.ForZones) == 0 {
					useTopologyHints = false
					break
				}
			}
			if useTopologyHints {
				log.Info().Msgf("All endpoint slices has zone hint, using zone %q for Service %q", zoneForHints, svcKey)
			}
		}

		for _, ep := range eps.Endpoints {
			if (ep.Conditions.Ready != nil) && !(*ep.Conditions.Ready) {
				continue
			}
			epHasZone := false
			if useTopologyHints {
				for _, epzone := range ep.Hints.ForZones {
					if epzone.Name == zoneForHints {
						epHasZone = true
						break
					}
				}
			}

			if useTopologyHints && !epHasZone {
				continue
			}

			for _, epPort := range ports {
				for _, epAddress := range ep.Addresses {
					hostPort := net.JoinHostPort(epAddress, strconv.Itoa(int(epPort)))
					if _, exists := processedUpstreamServers[hostPort]; exists {
						continue
					}
					ups := Endpoint{
						Address: epAddress,
						Port:    fmt.Sprintf("%v", epPort),
						Target:  ep.TargetRef,
					}
					upsServers = append(upsServers, ups)
					processedUpstreamServers[hostPort] = struct{}{}
				}
			}
		}
	}

	log.Info().Msgf("Endpoints found for Service %q: %v", svcKey, upsServers)
	return upsServers
}
