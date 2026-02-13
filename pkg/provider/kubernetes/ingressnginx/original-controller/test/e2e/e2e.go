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

package e2e

import (
	"os"
	"testing"

	"github.com/onsi/ginkgo/v2"
	"k8s.io/component-base/logs"

	// required
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original-controller/test/e2e/framework"

	// tests to run
	_ "github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original-controller/test/e2e/admission"
	_ "github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original-controller/test/e2e/annotations"
	_ "github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original-controller/test/e2e/annotations/modsecurity"
	_ "github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original-controller/test/e2e/cgroups"
	_ "github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original-controller/test/e2e/dbg"
	_ "github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original-controller/test/e2e/defaultbackend"
	_ "github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original-controller/test/e2e/disableleaderelection"
	_ "github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original-controller/test/e2e/endpointslices"
	_ "github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original-controller/test/e2e/gracefulshutdown"
	_ "github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original-controller/test/e2e/ingress"
	_ "github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original-controller/test/e2e/leaks"
	_ "github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original-controller/test/e2e/loadbalance"
	_ "github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original-controller/test/e2e/lua"
	_ "github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original-controller/test/e2e/metrics"
	_ "github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original-controller/test/e2e/nginx"
	_ "github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original-controller/test/e2e/security"
	_ "github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original-controller/test/e2e/servicebackend"
	_ "github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original-controller/test/e2e/settings"
	_ "github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original-controller/test/e2e/settings/modsecurity"
	_ "github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original-controller/test/e2e/settings/ocsp"
	_ "github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original-controller/test/e2e/settings/validations"
	_ "github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original-controller/test/e2e/ssl"
	_ "github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original-controller/test/e2e/status"
	_ "github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingressnginx/original-controller/test/e2e/tcpudp"
)

// RunE2ETests checks configuration parameters (specified through flags) and then runs
// E2E tests using the Ginkgo runner.
func RunE2ETests(t *testing.T) {
	logs.InitLogs()
	defer logs.FlushLogs()

	if os.Getenv("KUBECTL_PATH") != "" {
		framework.KubectlPath = os.Getenv("KUBECTL_PATH")
		framework.Logf("Using kubectl path '%s'", framework.KubectlPath)
	}

	framework.Logf("Starting e2e run %q on Ginkgo node %d", framework.RunID, ginkgo.GinkgoParallelProcess())
	ginkgo.RunSpecs(t, "nginx-ingress-controller e2e suite")
}
