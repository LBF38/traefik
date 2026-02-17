package ingressnginx

import (
	"strings"

	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original-controller/controller/ingress/annotations/auth"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original-controller/controller/ingress/annotations/authreq"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original-controller/controller/ingress/annotations/authtls"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original-controller/controller/ingress/annotations/canary"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original-controller/controller/ingress/annotations/connection"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original-controller/controller/ingress/annotations/cors"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original-controller/controller/ingress/annotations/customheaders"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original-controller/controller/ingress/annotations/fastcgi"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original-controller/controller/ingress/annotations/ipallowlist"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original-controller/controller/ingress/annotations/ipdenylist"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original-controller/controller/ingress/annotations/log"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original-controller/controller/ingress/annotations/mirror"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original-controller/controller/ingress/annotations/modsecurity"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original-controller/controller/ingress/annotations/opentelemetry"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original-controller/controller/ingress/annotations/proxy"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original-controller/controller/ingress/annotations/proxyssl"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original-controller/controller/ingress/annotations/ratelimit"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original-controller/controller/ingress/annotations/redirect"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original-controller/controller/ingress/annotations/rewrite"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original-controller/controller/ingress/annotations/sessionaffinity"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original-controller/controller/ingress/annotations/sslcipher"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original-controller/controller/ingress/annotations/upstreamhashby"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original-controller/controller/ingress/resolver"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

type Ingress struct {
	*netv1.Ingress
	ParsedAnnotations      *ingressConfig
	ParsedAnnotationsNGINX *ParsedAnnotationsNGINX
}

// Ingress defines the valid annotations present in one NGINX Ingress rule
type ParsedAnnotationsNGINX struct {
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
	DefaultBackend              *corev1.Service
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

func toAnnotations(src *ingressConfig) *ParsedAnnotationsNGINX {
	// TODO: finish mapping
	return &ParsedAnnotationsNGINX{
		BackendProtocol: "",
		Aliases:         []string{},
		BasicDigestAuth: auth.Config{
			Type:       ptr.Deref(src.AuthType, ""),
			Realm:      ptr.Deref(src.AuthRealm, ""),
			Secret:     ptr.Deref(src.AuthSecret, ""),
			SecretType: ptr.Deref(src.AuthSecretType, ""),
		},
		Canary: canary.Config{
			Enabled:       ptr.Deref(src.Canary, false),
			Weight:        ptr.Deref(src.CanaryWeight, 0),
			WeightTotal:   ptr.Deref(src.CanaryWeightTotal, 100),
			Header:        ptr.Deref(src.CanaryByHeader, ""),
			HeaderValue:   ptr.Deref(src.CanaryByHeaderValue, ""),
			HeaderPattern: ptr.Deref(src.CanaryByHeaderPattern, ""),
			Cookie:        ptr.Deref(src.CanaryByCookie, ""),
		},
		CertificateAuth: authtls.Config{
			AuthSSLCert:        resolver.AuthSSLCert{},
			VerifyClient:       ptr.Deref(src.AuthTLSVerifyClient, ""),
			ValidationDepth:    0,
			ErrorPage:          "",
			PassCertToUpstream: false,
			MatchCN:            "",
			AuthTLSError:       "",
		},
		ClientBodyBufferSize: "",
		CustomHeaders:        customheaders.Config{},
		ConfigurationSnippet: "",
		Connection:           connection.Config{},
		CorsConfig: cors.Config{
			CorsEnabled:          ptr.Deref(src.EnableCORS, false),
			CorsAllowCredentials: ptr.Deref(src.EnableCORSAllowCredentials, false),
			CorsExposeHeaders:    strings.Join(ptr.Deref(src.CORSExposeHeaders, []string{}), ","),
		},
		CustomHTTPErrors:            []int{},
		DisableProxyInterceptErrors: false,
		DefaultBackend:              &corev1.Service{},
		FastCGI:                     fastcgi.Config{},
		Denied:                      new(string),
		ExternalAuth:                authreq.Config{},
		EnableGlobalAuth:            false,
		HTTP2PushPreload:            false,
		Opentelemetry:               opentelemetry.Config{},
		Proxy:                       proxy.Config{},
		ProxySSL:                    proxyssl.Config{},
		RateLimit:                   ratelimit.Config{},
		Redirect:                    redirect.Config{},
		Rewrite:                     rewrite.Config{},
		Satisfy:                     "",
		ServerSnippet:               "",
		ServiceUpstream:             false,
		SessionAffinity:             sessionaffinity.Config{},
		SSLPassthrough:              false,
		UsePortInRedirects:          false,
		UpstreamHashBy:              upstreamhashby.Config{},
		LoadBalancing:               "",
		UpstreamVhost:               ptr.Deref(src.UpstreamVhost, ""),
		Denylist:                    ipdenylist.SourceRange{},
		XForwardedPrefix:            "",
		SSLCipher:                   sslcipher.Config{},
		Logs:                        log.Config{},
		ModSecurity:                 modsecurity.Config{},
		Mirror:                      mirror.Config{},
		StreamSnippet:               "",
		Allowlist:                   ipallowlist.SourceRange{},
	}
}

// Server describes a website
type Server struct {
	// Hostname returns the FQDN of the server
	Hostname string `json:"hostname"`
	// SSLPassthrough indicates if the TLS termination is realized in
	// the server or in the remote endpoint
	SSLPassthrough bool `json:"sslPassthrough"`
	// SSLCert describes the certificate that will be used on the server
	SSLCert *SSLCert `json:"sslCert"`
	// Locations list of URIs configured in the server.
	Locations []*Location `json:"locations,omitempty"`
	// Aliases return the alias of the server name
	Aliases []string `json:"aliases,omitempty"`
	// RedirectFromToWWW returns if a redirect to/from prefix www is required
	RedirectFromToWWW bool `json:"redirectFromToWWW,omitempty"`
	// CertificateAuth indicates this server requires mutual authentication
	// +optional
	CertificateAuth authtls.Config `json:"certificateAuth"`
	// ProxySSL indicates this server uses client certificate to access backends
	// +optional
	ProxySSL proxyssl.Config `json:"proxySSL"`
	// ServerSnippet returns the snippet of server
	// +optional
	ServerSnippet string `json:"serverSnippet"`
	// SSLCiphers returns list of ciphers to be enabled
	SSLCiphers string `json:"sslCiphers,omitempty"`
	// SSLPreferServerCiphers indicates that server ciphers should be preferred
	// over client ciphers when using the TLS protocols.
	SSLPreferServerCiphers string `json:"sslPreferServerCiphers,omitempty"`
	// AuthTLSError contains the reason why the access to a server should be denied
	AuthTLSError string `json:"authTLSError,omitempty"`
}

// Location describes an URI inside a server.
// Also contains additional information about annotations in the Ingress.
//
// In some cases when more than one annotation is defined a particular order in the execution
// is required.
// The chain in the execution order of annotations should be:
// - Denylist
// - Allowlist
// - RateLimit
// - BasicDigestAuth
// - ExternalAuth
// - Redirect
type Location struct {
	// Path is an extended POSIX regex as defined by IEEE Std 1003.1,
	// (i.e this follows the egrep/unix syntax, not the perl syntax)
	// matched against the path of an incoming request. Currently it can
	// contain characters disallowed from the conventional "path"
	// part of a URL as defined by RFC 3986. Paths must begin with
	// a '/'. If unspecified, the path defaults to a catch all sending
	// traffic to the backend.
	Path string `json:"path"`
	// PathType represents the type of path referred to by a HTTPIngressPath.
	PathType *netv1.PathType `json:"pathType"`
	// IsDefBackend indicates if service specified in the Ingress
	// contains active endpoints or not. Returning true means the location
	// uses the default backend.
	IsDefBackend bool `json:"isDefBackend"`
	// Ingress returns the ingress from which this location was generated
	Ingress *Ingress `json:"ingress"`
	// IngressPath original path defined in the ingress rule
	IngressPath string `json:"ingressPath"`
	// Backend describes the name of the backend to use.
	Backend string `json:"backend"`
	// Service describes the referenced services from the ingress
	Service *corev1.Service `json:"-"`
	// Port describes to which port from the service
	Port intstr.IntOrString `json:"port"`
	// Overwrite the Host header passed into the backend. Defaults to
	// vhost of the incoming request.
	// +optional
	UpstreamVhost string `json:"upstream-vhost"`
	// BasicDigestAuth returns authentication configuration for
	// an Ingress rule.
	// +optional
	BasicDigestAuth auth.Config `json:"basicDigestAuth,omitempty"`
	// Denied returns an error when this location cannot not be allowed
	// Requesting a denied location should return HTTP code 403.
	Denied        *string              `json:"denied,omitempty"`
	CustomHeaders customheaders.Config `json:"customHeaders,omitempty"`
	// CorsConfig returns the Cors Configuration for the ingress rule
	// +optional
	CorsConfig cors.Config `json:"corsConfig,omitempty"`
	// ExternalAuth indicates the access to this location requires
	// authentication using an external provider
	// +optional
	ExternalAuth authreq.Config `json:"externalAuth,omitempty"`
	// EnableGlobalAuth indicates if the access to this location requires
	// authentication using an external provider defined in controller's config
	EnableGlobalAuth bool `json:"enableGlobalAuth"`
	// HTTP2PushPreload allows to configure the HTTP2 Push Preload from backend
	// original location.
	// +optional
	HTTP2PushPreload bool `json:"http2PushPreload,omitempty"`
	// RateLimit describes a limit in the number of connections per IP
	// address or connections per second.
	// The Redirect annotation precedes RateLimit
	// +optional
	RateLimit ratelimit.Config `json:"rateLimit,omitempty"`
	// Redirect describes a temporal o permanent redirection this location.
	// +optional
	Redirect redirect.Config `json:"redirect,omitempty"`
	// Rewrite describes the redirection this location.
	// +optional
	Rewrite rewrite.Config `json:"rewrite,omitempty"`
	// Denylist indicates only connections from certain client
	// addresses or networks are allowed.
	// +optional
	Denylist ipdenylist.SourceRange `json:"denylist,omitempty"`
	// Allowlist indicates only connections from certain client
	// addresses or networks are allowed.
	// +optional
	Allowlist ipallowlist.SourceRange `json:"allowlist,omitempty"`
	// Proxy contains information about timeouts and buffer sizes
	// to be used in connections against endpoints
	// +optional
	Proxy proxy.Config `json:"proxy,omitempty"`
	// ProxySSL contains information about SSL configuration parameters
	// to be used in connections against endpoints
	// +optional
	ProxySSL proxyssl.Config `json:"proxySSL,omitempty"`
	// UsePortInRedirects indicates if redirects must specify the port
	// +optional
	UsePortInRedirects bool `json:"usePortInRedirects"`
	// ConfigurationSnippet contains additional configuration for the backend
	// to be considered in the configuration of the location
	ConfigurationSnippet string `json:"configurationSnippet"`
	// Connection contains connection header to override the default Connection header
	// to the request.
	// +optional
	Connection connection.Config `json:"connection"`
	// ClientBodyBufferSize allows for the configuration of the client body
	// buffer size for a specific location.
	// +optional
	ClientBodyBufferSize string `json:"clientBodyBufferSize,omitempty"`
	// DefaultBackend allows the use of a custom default backend for this location.
	// +optional
	DefaultBackend *corev1.Service `json:"-"`
	// DefaultBackendUpstreamName is the upstream-formatted string for the name of
	// this location's custom default backend
	DefaultBackendUpstreamName string `json:"defaultBackendUpstreamName,omitempty"`
	// XForwardedPrefix allows to add a header X-Forwarded-Prefix to the request with the
	// original location.
	// +optional
	XForwardedPrefix string `json:"xForwardedPrefix,omitempty"`
	// Logs allows to enable or disable the nginx logs
	// By default access logs are enabled and rewrite logs are disabled
	Logs log.Config `json:"logs,omitempty"`
	// BackendProtocol indicates which protocol should be used to communicate with the service
	// By default this is HTTP
	BackendProtocol string `json:"backend-protocol"`
	// FastCGI allows the ingress to act as a FastCGI client for a given location.
	// +optional
	FastCGI fastcgi.Config `json:"fastcgi,omitempty"`
	// CustomHTTPErrors specifies the error codes that should be intercepted.
	// +optional
	CustomHTTPErrors []int `json:"custom-http-errors"`
	// ProxyInterceptErrors disables error interception when using CustomHTTPErrors
	// e.g. custom 404 and 503 when service-a does not exist or is not available
	// but service-a can return 404 and 503 error codes without intercept
	// +optional
	DisableProxyInterceptErrors bool `json:"disable-proxy-intercept-errors"`
	// ModSecurity allows to enable and configure modsecurity
	// +optional
	ModSecurity modsecurity.Config `json:"modsecurity"`
	// Satisfy dictates allow access if any or all is set
	Satisfy string `json:"satisfy"`
	// Mirror allows you to mirror traffic to a "test" backend
	// +optional
	Mirror mirror.Config `json:"mirror,omitempty"`
	// Opentelemetry allows the global opentelemetry setting to be overridden for a location
	// +optional
	Opentelemetry opentelemetry.Config `json:"opentelemetry"`
}

// TODO
type NginxConfiguration struct {
	Backends            []*Backend
	Servers             []*Server
	PassthroughBackends []*SSLPassthroughBackend
}

// Backend describes one or more remote server/s (endpoints) associated with a service
// +k8s:deepcopy-gen=true
type Backend struct {
	// Name represents an unique apiv1.Service name formatted as <namespace>-<name>-<port>
	Name    string             `json:"name"`
	Service *corev1.Service    `json:"service,omitempty"`
	Port    intstr.IntOrString `json:"port"`
	// SSLPassthrough indicates that Ingress controller will delegate TLS termination to the endpoints.
	SSLPassthrough bool `json:"sslPassthrough"`
	// Endpoints contains the list of endpoints currently running
	Endpoints []Endpoint `json:"endpoints,omitempty"`
	// StickySessionAffinitySession contains the StickyConfig object with stickiness configuration
	SessionAffinity SessionAffinityConfig `json:"sessionAffinityConfig"`
	// Consistent hashing by NGINX variable
	UpstreamHashBy UpstreamHashByConfig `json:"upstreamHashByConfig"`
	// LB algorithm configuration per ingress
	LoadBalancing string `json:"load-balance,omitempty"`
	// Denotes if a backend has no server. The backend instead shares a server with another backend and acts as an
	// alternative backend.
	// This can be used to share multiple upstreams in the sam nginx server block.
	NoServer bool `json:"noServer"`
	// Policies to describe the characteristics of an alternative backend.
	// +optional
	TrafficShapingPolicy TrafficShapingPolicy `json:"trafficShapingPolicy"`
	// Contains a list of backends without servers that are associated with this backend.
	// +optional
	AlternativeBackends []string `json:"alternativeBackends,omitempty"`
}

// UpstreamHashByConfig described setting from the upstream-hash-by* annotations.
type UpstreamHashByConfig struct {
	UpstreamHashBy           string `json:"upstream-hash-by,omitempty"`
	UpstreamHashBySubset     bool   `json:"upstream-hash-by-subset,omitempty"`
	UpstreamHashBySubsetSize int    `json:"upstream-hash-by-subset-size,omitempty"`
}

// Endpoint describes a kubernetes endpoint in a backend
// +k8s:deepcopy-gen=true
type Endpoint struct {
	// Address IP address of the endpoint
	Address string `json:"address"`
	// Port number of the TCP port
	Port string `json:"port"`
	// Target returns a reference to the object providing the endpoint
	Target *corev1.ObjectReference `json:"target,omitempty"`
}

// TrafficShapingPolicy describes the policies to put in place when a backend has no server and is used as an
// alternative backend
// +k8s:deepcopy-gen=true
type TrafficShapingPolicy struct {
	// Weight (0-<WeightTotal>) of traffic to redirect to the backend.
	// e.g. <WeightTotal> defaults to 100, weight 20 means 20% of traffic will be
	// redirected to the backend and 80% will remain with the other backend. If
	// <WeightTotal> is set to 1000, weight 2 means 0.2% of traffic will be
	// redirected to the backend and 99.8% will remain with the other backend.
	// 0 weight will not send any traffic to this backend
	Weight int `json:"weight"`
	// The total weight of traffic (>= 100). If unspecified, it defaults to 100.
	WeightTotal int `json:"weightTotal"`
	// Header on which to redirect requests to this backend
	Header string `json:"header"`
	// HeaderValue on which to redirect requests to this backend
	HeaderValue string `json:"headerValue"`
	// HeaderPattern the header value match pattern, support exact, regex.
	HeaderPattern string `json:"headerPattern"`
	// Cookie on which to redirect requests to this backend
	Cookie string `json:"cookie"`
}

// SessionAffinityConfig describes different affinity configurations for new sessions.
// Once a session is mapped to a backend based on some affinity setting, it
// retains that mapping till the backend goes down, or the ingress controller
// restarts. Exactly one of these values will be set on the upstream, since multiple
// affinity values are incompatible. Once set, the backend makes no guarantees
// about honoring updates.
// +k8s:deepcopy-gen=true
type SessionAffinityConfig struct {
	AffinityType          string                `json:"name"`
	AffinityMode          string                `json:"mode"`
	CookieSessionAffinity CookieSessionAffinity `json:"cookieSessionAffinity"`
}

// CookieSessionAffinity defines the structure used in Affinity configured by Cookies.
// +k8s:deepcopy-gen=true
type CookieSessionAffinity struct {
	Name                    string              `json:"name"`
	Expires                 string              `json:"expires,omitempty"`
	MaxAge                  string              `json:"maxage,omitempty"`
	Locations               map[string][]string `json:"locations,omitempty"`
	Secure                  bool                `json:"secure,omitempty"`
	Path                    string              `json:"path,omitempty"`
	Domain                  string              `json:"domain,omitempty"`
	SameSite                string              `json:"samesite,omitempty"`
	ConditionalSameSiteNone bool                `json:"conditional_samesite_none,omitempty"`
	ChangeOnFailure         bool                `json:"change_on_failure,omitempty"`
}

// SSLPassthroughBackend describes a SSL upstream server configured
// as passthrough (no TLS termination in the ingress controller)
// The endpoints must provide the TLS termination exposing the required SSL certificate.
// The ingress controller only pipes the underlying TCP connection
type SSLPassthroughBackend struct {
	Service *corev1.Service    `json:"-"`
	Port    intstr.IntOrString `json:"port"`
	// Backend describes the endpoints to use.
	Backend string `json:"namespace,omitempty"`
	// Hostname returns the FQDN of the server
	Hostname string `json:"hostname"`
}
