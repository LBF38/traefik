package ingressnginx

import (
	"fmt"
	"sort"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/controller/k8s"
	netv1 "k8s.io/api/networking/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
)

const (
	rootLocation    = "/"
	defUpstreamName = "upstream-default-backend"
	defServerName   = "_"
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
		// TCPEndpoints:          n.getStreamServices(n.cfg.TCPConfigMapName, apiv1.ProtocolTCP),
		// UDPEndpoints:          n.getStreamServices(n.cfg.UDPConfigMapName, apiv1.ProtocolUDP),
		PassthroughBackends: passUpstreams,
		// BackendConfigChecksum: n.store.GetBackendConfiguration().Checksum,
		// DefaultSSLCertificate: n.getDefaultSSLCertificate(),
		// StreamSnippets:        n.getStreamSnippets(ingresses),
	}
}

// TODO
func (p *Provider) getBackendServers(ingresses []*Ingress) ([]*Backend, []*Server) {
	// du := getDefaultUpstream()
	// upstreams := createUpstreams(ingresses, du)
	// servers := createServers(ingresses, upstreams, du)
	var upstreams map[string]*Backend
	var servers map[string]*Server

	var canaryIngresses []*Ingress

	for _, ing := range ingresses {
		ingKey := k8s.MetaNamespaceKey(ing)
		anns := ing.ParsedAnnotations

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

			// if server.AuthTLSError == "" && anns.CertificateAuth.AuthTLSError != "" {
			// 	server.AuthTLSError = anns.CertificateAuth.AuthTLSError
			// }

			// if server.CertificateAuth.CAFileName == "" {
			// 	server.CertificateAuth = anns.CertificateAuth
			// 	if server.CertificateAuth.Secret != "" && server.CertificateAuth.CAFileName == "" {
			// 		log.Info().Msgf("Secret %q has no 'ca.crt' key, mutual authentication disabled for Ingress %q",
			// 			server.CertificateAuth.Secret, ingKey)
			// 	}
			// } else {
			// 	log.Info().Msgf("Server %q is already configured for mutual authentication (Ingress %q)",
			// 		server.Hostname, ingKey)
			// }

			// if !p.ProxySSLLocationOnly {
			// 	if server.ProxySSL.CAFileName == "" {
			// 		server.ProxySSL = anns.ProxySSL
			// 		if server.ProxySSL.Secret != "" && server.ProxySSL.CAFileName == "" {
			// 			log.Info().Msgf("Secret %q has no 'ca.crt' key, client cert authentication disabled for Ingress %q",
			// 				server.ProxySSL.Secret, ingKey)
			// 		}
			// 	} else {
			// 		log.Info().Msgf("Server %q is already configured for client cert authentication (Ingress %q)",
			// 			server.Hostname, ingKey)
			// 	}
			// }

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

				// if ups.SessionAffinity.AffinityType == "" {
				// 	ups.SessionAffinity.AffinityType = anns.SessionAffinity.Type
				// }

				// if ups.SessionAffinity.AffinityMode == "" {
				// 	ups.SessionAffinity.AffinityMode = anns.SessionAffinity.Mode
				// }

				// if anns.SessionAffinity.Type == "cookie" {
				// 	cookiePath := anns.SessionAffinity.Cookie.Path
				// 	if anns.Rewrite.UseRegex && cookiePath == "" {
				// 	log.Warn().Msgf("session-cookie-path should be set when use-regex is true")
				// 	}

				// 	ups.SessionAffinity.CookieSessionAffinity.Name = anns.SessionAffinity.Cookie.Name
				// 	ups.SessionAffinity.CookieSessionAffinity.Expires = anns.SessionAffinity.Cookie.Expires
				// 	ups.SessionAffinity.CookieSessionAffinity.MaxAge = anns.SessionAffinity.Cookie.MaxAge
				// 	ups.SessionAffinity.CookieSessionAffinity.Secure = anns.SessionAffinity.Cookie.Secure
				// 	ups.SessionAffinity.CookieSessionAffinity.Path = cookiePath
				// 	ups.SessionAffinity.CookieSessionAffinity.Domain = anns.SessionAffinity.Cookie.Domain
				// 	ups.SessionAffinity.CookieSessionAffinity.SameSite = anns.SessionAffinity.Cookie.SameSite
				// 	ups.SessionAffinity.CookieSessionAffinity.ConditionalSameSiteNone = anns.SessionAffinity.Cookie.ConditionalSameSiteNone
				// 	ups.SessionAffinity.CookieSessionAffinity.ChangeOnFailure = anns.SessionAffinity.Cookie.ChangeOnFailure

				// 	locs := ups.SessionAffinity.CookieSessionAffinity.Locations
				// 	if _, ok := locs[host]; !ok {
				// 		locs[host] = []string{}
				// 	}
				// 	locs[host] = append(locs[host], path.Path)

				// 	if len(server.Aliases) > 0 {
				// 		for _, alias := range server.Aliases {
				// 			if _, ok := locs[alias]; !ok {
				// 				locs[alias] = []string{}
				// 			}
				// 			locs[alias] = append(locs[alias], path.Path)
				// 		}
				// 	}
				// }
			}
		}

		// // set aside canary ingresses to merge later
		// if anns.Canary.Enabled {
		// 	canaryIngresses = append(canaryIngresses, ing)
		// }
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
					klog.Errorf("Custom default backend service %v/%v has no ports. Ignoring", location.DefaultBackend.Namespace, location.DefaultBackend.Name)
					continue
				}

				// sp := location.DefaultBackend.Spec.Ports[0]
				// var zone string
				// if p.EnableTopologyAwareRouting {
				// 	zone = getIngressPodZone(location.DefaultBackend)
				// } else {
				// 	zone = emptyZone
				// }
				// endps := getEndpointsFromSlices(location.DefaultBackend, &sp, corev1.ProtocolTCP, zone, p.GetServiceEndpointsSlices)
				// // custom backend is valid only if contains at least one endpoint
				// if len(endps) > 0 {
				// 	name := fmt.Sprintf("custom-default-backend-%v-%v", location.DefaultBackend.GetNamespace(), location.DefaultBackend.GetName())
				// 	log.Info().Msgf("Creating \"%v\" upstream based on default backend annotation", name)

				// 	nb := upstream.DeepCopy()
				// 	nb.Name = name
				// 	nb.Endpoints = endps
				// 	aUpstreams = append(aUpstreams, nb)
				// 	location.DefaultBackendUpstreamName = name

				// 	if len(upstream.Endpoints) == 0 {
				// 		log.Info().Msgf("Upstream %q has no active Endpoint, so using custom default backend for location %q in server %q (Service \"%v/%v\")",
				// 			upstream.Name, location.Path, server.Hostname, location.DefaultBackend.Namespace, location.DefaultBackend.Name)

				// 		location.Backend = name
				// 	}
				// }

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

func locationApplyAnnotations(loc *Location, anns *ingressConfig) {
	// loc.BasicDigestAuth = anns.BasicDigestAuth
	// loc.ClientBodyBufferSize = anns.ClientBodyBufferSize
	// loc.CustomHeaders = anns.CustomHeaders
	// loc.ConfigurationSnippet = anns.ConfigurationSnippet
	// loc.CorsConfig = anns.CorsConfig
	// loc.ExternalAuth = anns.ExternalAuth
	// loc.EnableGlobalAuth = anns.EnableGlobalAuth
	// loc.HTTP2PushPreload = anns.HTTP2PushPreload
	// loc.Opentelemetry = anns.Opentelemetry
	// loc.Proxy = anns.Proxy
	// loc.ProxySSL = anns.ProxySSL
	// loc.RateLimit = anns.RateLimit
	// loc.Redirect = anns.Redirect
	// loc.Rewrite = anns.Rewrite
	// loc.UpstreamVhost = anns.UpstreamVhost
	// loc.Denylist = anns.Denylist
	// loc.Allowlist = anns.Allowlist
	// loc.Denied = anns.Denied
	// loc.XForwardedPrefix = anns.XForwardedPrefix
	// loc.UsePortInRedirects = anns.UsePortInRedirects
	// loc.Connection = anns.Connection
	// loc.Logs = anns.Logs
	// loc.DefaultBackend = anns.DefaultBackend
	// loc.BackendProtocol = anns.BackendProtocol
	// loc.FastCGI = anns.FastCGI
	// loc.CustomHTTPErrors = anns.CustomHTTPErrors
	// loc.DisableProxyInterceptErrors = anns.DisableProxyInterceptErrors
	// loc.ModSecurity = anns.ModSecurity
	// loc.Satisfy = anns.Satisfy
	// loc.Mirror = anns.Mirror

	loc.DefaultBackendUpstreamName = defUpstreamName
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

func dropSnippetDirectives(anns *ingressConfig, ingKey string) {
	// TODO: WIP

	// 	if anns != nil {
	// 		if anns.ConfigurationSnippet != "" {
	// 			klog.V(3).Infof("Ingress %q tried to use configuration-snippet and the annotation is disabled by the admin. Removing the annotation", ingKey)
	// 			anns.ConfigurationSnippet = ""
	// 		}
	// 		if anns.ServerSnippet != "" {
	// 			klog.V(3).Infof("Ingress %q tried to use server-snippet and the annotation is disabled by the admin. Removing the annotation", ingKey)
	// 			anns.ServerSnippet = ""
	// 		}

	// 		if anns.ModSecurity.Snippet != "" {
	// 			klog.V(3).Infof("Ingress %q tried to use modsecurity-snippet and the annotation is disabled by the admin. Removing the annotation", ingKey)
	// 			anns.ModSecurity.Snippet = ""
	// 		}

	// 		if anns.ExternalAuth.AuthSnippet != "" {
	// 			klog.V(3).Infof("Ingress %q tried to use auth-snippet and the annotation is disabled by the admin. Removing the annotation", ingKey)
	// 			anns.ExternalAuth.AuthSnippet = ""
	// 		}

	//		if anns.StreamSnippet != "" {
	//			klog.V(3).Infof("Ingress %q tried to use stream-snippet and the annotation is disabled by the admin. Removing the annotation", ingKey)
	//			anns.StreamSnippet = ""
	//		}
	//	}
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
			klog.V(2).Infof("skip merge alternative backend %v into %v, it's already present", altUps.Name, priUps.Name)
			return true
		}
	}

	// if ing.ParsedAnnotations != nil && ing.ParsedAnnotations.SessionAffinity.CanaryBehavior != "legacy" {
	// 	priUps.SessionAffinity.DeepCopyInto(&altUps.SessionAffinity)
	// }

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
					klog.V(2).Infof("matching backend %v found for alternative backend %v",
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
				klog.V(3).Infof("Ingress %q and path %q does not contain a service backend, using default backend", k8s.MetaNamespaceKey(ing), path.Path)
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
				klog.Errorf("cannot merge alternative backend %s into hostname %s that does not exist",
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
					klog.V(2).Infof("matching backend %v found for alternative backend %v",
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
