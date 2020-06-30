// Package app provides flags for cli.
package app

import (
	pmmversion "github.com/percona/pmm/version"
	"github.com/pkg/errors"
	"gopkg.in/alecthomas/kingpin.v2"
)

// ErrNoName is an error in case name is not provided.
var ErrNoName = errors.New("app.Setup: no Name")

// Flags contains flags for cli.
type Flags struct {
	// gRPC listen address
	GRPCAddr string
	// Debug listen address
	DebugAddr string
}

func version() string {
	return pmmversion.PMMVersion
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
	kingpin.Version(version())
	kingpin.HelpFlag.Short('h')

	var flags Flags
	kingpin.Flag("grpc.addr", "gRPC listen address").Default(":20201").StringVar(&flags.GRPCAddr)
	kingpin.Flag("debug.addr", "Debug listen address").Default(":20203").StringVar(&flags.DebugAddr)

	return &flags, nil
}
