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
}

// SetupOpts contains options required for app.
type SetupOpts struct {
	Name string
}

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

	return &flags, nil
}
