/*
Copyright 2019 The Kubernetes Authors.

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

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"k8s.io/cli-runtime/pkg/genericclioptions"

	// Just importing this is supposed to allow cloud authentication
	// eg GCP, AWS, Azure ...
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/cmd/plugin/commands/backends"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/cmd/plugin/commands/certs"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/cmd/plugin/commands/conf"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/cmd/plugin/commands/exec"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/cmd/plugin/commands/general"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/cmd/plugin/commands/info"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/cmd/plugin/commands/ingresses"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/cmd/plugin/commands/lint"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/cmd/plugin/commands/logs"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/ingress-nginx/original-controller/cmd/plugin/commands/ssh"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "ingress-nginx",
		Short: "A kubectl plugin for inspecting your ingress-nginx deployments",
	}

	// Respect some basic kubectl flags like --namespace
	flags := genericclioptions.NewConfigFlags(true)
	flags.AddFlags(rootCmd.PersistentFlags())

	rootCmd.AddCommand(ingresses.CreateCommand(flags))
	rootCmd.AddCommand(conf.CreateCommand(flags))
	rootCmd.AddCommand(general.CreateCommand(flags))
	rootCmd.AddCommand(backends.CreateCommand(flags))
	rootCmd.AddCommand(info.CreateCommand(flags))
	rootCmd.AddCommand(certs.CreateCommand(flags))
	rootCmd.AddCommand(logs.CreateCommand(flags))
	rootCmd.AddCommand(exec.CreateCommand(flags))
	rootCmd.AddCommand(ssh.CreateCommand(flags))
	rootCmd.AddCommand(lint.CreateCommand(flags))

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
