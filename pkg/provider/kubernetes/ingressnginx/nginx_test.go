package ingressnginx

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

func TestModel(t *testing.T) {
	tests := []struct {
		desc                           string
		defaultBackendServiceName      string
		defaultBackendServiceNamespace string
		paths                          []string
		expectedHosts                  []string
		expectedConfiguration          NginxConfiguration
		expectedServers                []*Server
	}{
		{
			desc: "Custom Headers",
			paths: []string{
				"services.yml",
				"ingressclasses.yml",
				"ingresses/ingress-with-custom-headers.yml",
			},
			expectedHosts: []string{"whoami.localhost"},
			expectedServers: []*Server{
				{
					Hostname: "whoami.localhost",
					Locations: []*Location{
						{
							Path:     "/",
							PathType: &pathTypeExact,
						},
					},
				},
			},
			expectedConfiguration: NginxConfiguration{
				Backends: []*Backend{},
				Servers: []*Server{
					{
						Hostname: "whoami.localhost",
						Locations: []*Location{
							{
								Path:     "/",
								PathType: &pathTypeExact,
							},
						},
					},
				},
				PassthroughBackends: []*SSLPassthroughBackend{},
			},
		},
		{
			desc: "No annotation",
			paths: []string{
				"ingresses/ingress-with-no-annotation.yml",
				"ingressclasses.yml",
				"services.yml",
				"secrets.yml",
			},
		},
		{
			desc: "Basic Auth",
			paths: []string{
				"services.yml",
				"ingressclasses.yml",
				"ingresses/ingress-with-basicauth.yml",
			},
		},
		{
			desc: "Forward Auth",
			paths: []string{
				"services.yml",
				"ingressclasses.yml",
				"ingresses/ingress-with-forwardauth.yml",
			},
		},
		{
			desc: "SSL Redirect",
			paths: []string{
				"services.yml",
				"secrets.yml",
				"ingressclasses.yml",
				"ingresses/ingress-with-ssl-redirect.yml",
			},
		},
		{
			desc: "SSL Passthrough",
			paths: []string{
				"services.yml",
				"secrets.yml",
				"ingressclasses.yml",
				"ingresses/ingress-with-ssl-passthrough.yml",
			},
		},
		{
			desc: "Sticky Sessions",
			paths: []string{
				"services.yml",
				"secrets.yml",
				"ingressclasses.yml",
				"ingresses/ingress-with-sticky.yml",
			},
		},
		{
			desc: "Proxy SSL",
			paths: []string{
				"services.yml",
				"secrets.yml",
				"ingressclasses.yml",
				"ingresses/ingress-with-proxy-ssl.yml",
			},
		},
		{
			desc: "CORS",
			paths: []string{
				"services.yml",
				"ingressclasses.yml",
				"ingresses/ingress-with-cors.yml",
			},
		},
		{
			desc: "Service Upstream",
			paths: []string{
				"services.yml",
				"ingressclasses.yml",
				"ingresses/ingress-with-service-upstream.yml",
			},
		},
		{
			desc: "Upstream vhost",
			paths: []string{
				"services.yml",
				"ingressclasses.yml",
				"ingresses/ingress-with-upstream-vhost.yml",
			},
		},
		{
			desc: "Use Regex",
			paths: []string{
				"services.yml",
				"ingressclasses.yml",
				"ingresses/ingress-with-use-regex.yml",
			},
		},
		{
			desc: "Rewrite Target",
			paths: []string{
				"services.yml",
				"ingressclasses.yml",
				"ingresses/ingress-with-rewrite-target.yml",
			},
		},
		{
			desc: "App Root",
			paths: []string{
				"services.yml",
				"ingressclasses.yml",
				"ingresses/ingress-with-app-root.yml",
			},
		},
		{
			desc: "App Root - no prefix slash",
			paths: []string{
				"services.yml",
				"ingressclasses.yml",
				"ingresses/ingress-with-app-root-wrong.yml",
			},
		},
		{
			desc: "From To WWW Redirect - www host",
			paths: []string{
				"services.yml",
				"ingressclasses.yml",
				"ingresses/ingress-with-www-host.yml",
			},
		},
		{
			desc: "From To WWW Redirect - host",
			paths: []string{
				"services.yml",
				"ingressclasses.yml",
				"ingresses/ingress-with-host.yml",
			},
		},
		{
			desc: "From To WWW Redirect - multiple ingresses",
			paths: []string{
				"services.yml",
				"ingressclasses.yml",
				"ingresses/ingresses-with-www-redirect.yml",
			},
		},
		{
			desc:                           "Default Backend",
			defaultBackendServiceName:      "whoami",
			defaultBackendServiceNamespace: "default",
			paths: []string{
				"services.yml",
			},
		},
		{
			desc: "WhitelistSourceRange with single IP",
			paths: []string{
				"services.yml",
				"ingressclasses.yml",
				"ingresses/ingress-with-whitelist-single-ip.yml",
			},
		},
		{
			desc: "WhitelistSourceRange with single CIDR",
			paths: []string{
				"services.yml",
				"ingressclasses.yml",
				"ingresses/ingress-with-whitelist-single-cidr.yml",
			},
		},
		{
			desc: "WhitelistSourceRange when specified multiple IP/CIDR",
			paths: []string{
				"services.yml",
				"ingressclasses.yml",
				"ingresses/ingress-with-whitelist-multiple-ip-and-cidr.yml",
			},
		},
		{
			desc: "WhitelistSourceRange when empty ignored",
			paths: []string{
				"services.yml",
				"ingressclasses.yml",
				"ingresses/ingress-with-whitelist-empty.yml",
			},
		},
		{
			desc: "Permanent Redirect",
			paths: []string{
				"services.yml",
				"ingressclasses.yml",
				"ingresses/ingress-with-permanent-redirect.yml",
			},
		},
		{
			desc: "Permanent Redirect Code - wrong code",
			paths: []string{
				"services.yml",
				"ingressclasses.yml",
				"ingresses/ingress-with-permanent-redirect-code-wrong-code.yml",
			},
		},
		{
			desc: "Permanent Redirect Code - correct code",
			paths: []string{
				"services.yml",
				"ingressclasses.yml",
				"ingresses/ingress-with-permanent-redirect-code-correct-code.yml",
			},
		},
		{
			desc: "Temporal Redirect takes precedence over Permanent Redirect",
			paths: []string{
				"services.yml",
				"ingressclasses.yml",
				"ingresses/ingress-with-temporal-and-permanent-redirect.yml",
			},
		},
		{
			desc: "Temporal Redirect",
			paths: []string{
				"services.yml",
				"ingressclasses.yml",
				"ingresses/ingress-with-temporal-redirect.yml",
			},
		},
		{
			desc: "Temporal Redirect Code - wrong code",
			paths: []string{
				"services.yml",
				"ingressclasses.yml",
				"ingresses/ingress-with-temporal-redirect-code-wrong-code.yml",
			},
		},
		{
			desc: "Temporal Redirect Code - correct code",
			paths: []string{
				"services.yml",
				"ingressclasses.yml",
				"ingresses/ingress-with-temporal-redirect-code-correct-code.yml",
			},
		},
		{
			desc: "Proxy connect timeout",
			paths: []string{
				"services.yml",
				"ingressclasses.yml",
				"ingresses/ingress-with-proxy-timeout.yml",
			},
		},
		{
			desc: "Auth TLS secret",
			paths: []string{
				"services.yml",
				"secrets.yml",
				"ingressclasses.yml",
				"ingresses/ingress-with-auth-tls-secret.yml",
			},
		},
		{
			desc: "Auth TLS verify client",
			paths: []string{
				"services.yml",
				"secrets.yml",
				"ingressclasses.yml",
				"ingresses/ingress-with-auth-tls-verify-client.yml",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			k8sObjects := readResources(t, test.paths)
			kubeClient := kubefake.NewClientset(k8sObjects...)
			client := newClient(kubeClient)

			eventCh, err := client.WatchAll(t.Context(), "", "")
			require.NoError(t, err)

			if len(k8sObjects) > 0 {
				// just wait for the first event
				<-eventCh
			}

			p := Provider{
				k8sClient:                      client,
				defaultBackendServiceName:      test.defaultBackendServiceName,
				defaultBackendServiceNamespace: test.defaultBackendServiceNamespace,
			}
			p.SetDefaults()

			rawIngresses := p.getIngresses(t.Context())

			hosts, _, nginxConf := p.getNginxConfiguration(rawIngresses)
			assert.Equal(t, test.expectedHosts, hosts)
			// assert.Equal(t, test.expectedServers, servers)
			assert.Equal(t, test.expectedConfiguration, nginxConf)
		})
	}
}
