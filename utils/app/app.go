// dbaas-controller
// Copyright (C) 2020 Percona LLC
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

// Package app provides flags for cli.
package app

import (
	"fmt"

	"github.com/percona/pmm/version"
	"gopkg.in/alecthomas/kingpin.v2"
)

// ErrNoName is an error in case name is not provided.
var ErrNoName error = fmt.Errorf("app.Setup: no Name")

// Flags contains flags for cli.
type Flags struct {
	// gRPC listen address
	GRPCAddr string
	// Debug listen address
	DebugAddr string
	// PXCOperatorURLTemplate exists for user to fetch Kubernetes manifests when running DBaaS on air-gapped cluster.
	PXCOperatorURLTemplate string
	// PSMDBOperatorURLTemplate exists for user to fetch Kubernetes manifests when running DBaaS on air-gapped cluster.
	PSMDBOperatorURLTemplate string
	// Debug enabled.
	LogDebug bool
}

// SetupOpts contains options required for app.
type SetupOpts struct {
	Name string
}

const (
	// DefaultPXCOperatorURLTemplate is a URL template pointing at files needed to install/upgrade PXC operator.
	DefaultPXCOperatorURLTemplate = "https://raw.githubusercontent.com/percona/percona-xtradb-cluster-operator/v%s/deploy/%s"
	// DefaultPSMDBOperatorURLTemplate is a URL template pointing at files needed to install/upgrade PSMDB operator.
	DefaultPSMDBOperatorURLTemplate = "https://raw.githubusercontent.com/percona/percona-server-mongodb-operator/v%s/deploy/%s"
)

// Setup initialize app flags for cli.
func Setup(opts *SetupOpts) (*Flags, error) {
	if opts == nil {
		opts = new(SetupOpts)
	}

	if opts.Name == "" {
		return nil, ErrNoName
	}

	kingpin.CommandLine.Name = opts.Name
	kingpin.CommandLine.DefaultEnvars()
	kingpin.Version(version.FullInfo())
	kingpin.HelpFlag.Short('h')

	var flags Flags
	kingpin.Flag("grpc.addr", "gRPC listen address").Default(":20201").StringVar(&flags.GRPCAddr)
	kingpin.Flag("debug.addr", "Debug listen address").Default(":20203").StringVar(&flags.DebugAddr)
	kingpin.Flag(
		"pxc.operator.url.template",
		"URL template for fetching yaml manifests for Percona Kubernetes Operator for PXC. Place first '%s' into your URL where version should be placed and second '%s' for the yaml file.",
	).Default(
		DefaultPXCOperatorURLTemplate,
	).StringVar(&flags.PXCOperatorURLTemplate)
	kingpin.Flag(
		"psmdb.operator.url.template",
		"URL template for fetching yaml manifests for Percona Kubernetes Operator for PSMDB. Place first '%s' into your URL where version should be placed and second '%s' for the yaml file.",
	).Default(
		DefaultPSMDBOperatorURLTemplate,
	).StringVar(&flags.PSMDBOperatorURLTemplate)

	kingpin.Flag("debug", "Enable debug").Envar("PMM_DEBUG").BoolVar(&flags.LogDebug)

	return &flags, nil
}
