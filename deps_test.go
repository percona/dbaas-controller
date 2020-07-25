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

package dbaas_controller

import (
	"fmt"
	"go/build"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestImports(t *testing.T) {
	type constraint struct {
		blacklistPrefixes []string
	}

	constraints := make(map[string]constraint)

	// kubectl should not import Operator-specific APIs
	constraints["github.com/percona-platform/dbaas-controller/service/kubectl"] = constraint{
		blacklistPrefixes: []string{
			"github.com/percona/percona-server-mongodb-operator/pkg/apis",
			"github.com/percona/percona-xtradb-cluster-operator/pkg/apis",
		},
	}

	// cluster should not import kubectl
	constraints["github.com/percona-platform/dbaas-controller/service/cluster"] = constraint{
		blacklistPrefixes: []string{
			"github.com/percona-platform/dbaas-controller/service/kubectl",
		},
	}

	allImports := make(map[string]map[string]struct{})
	for path, c := range constraints {
		p, err := build.Import(path, ".", 0)
		require.NoError(t, err)

		if allImports[path] == nil {
			allImports[path] = make(map[string]struct{})
		}
		for _, i := range p.Imports {
			allImports[path][i] = struct{}{}
		}
		for _, i := range p.TestImports {
			allImports[path][i] = struct{}{}
		}
		for _, i := range p.XTestImports {
			allImports[path][i] = struct{}{}
		}

		for _, b := range c.blacklistPrefixes {
			for i := range allImports[path] {
				// whitelist own subpackages
				if strings.HasPrefix(i, path) {
					continue
				}

				// check blacklist
				if strings.HasPrefix(i, b) {
					t.Errorf("Package %q should not import package %q (blacklisted by %q).", path, i, b)
				}
			}
		}
	}

	f, err := os.Create("packages.dot")
	require.NoError(t, err)
	defer func() { require.NoError(t, f.Close()) }()

	fmt.Fprintf(f, "digraph packages {\n")

	packages := make([]string, 0, len(allImports))
	for p := range allImports {
		packages = append(packages, p)
	}
	sort.Strings(packages)

	for _, p := range packages {
		imports := make([]string, 0, len(allImports[p]))
		for p := range allImports[p] {
			imports = append(imports, p)
		}
		sort.Strings(imports)

		p = strings.TrimPrefix(p, "github.com/percona-platform/dbaas-controller")
		if p == "" {
			p = "/"
		}
		for _, i := range imports {
			if strings.Contains(i, "/utils/") {
				continue
			}
			if strings.HasPrefix(i, "github.com/percona-platform/dbaas-controller") {
				i = strings.TrimPrefix(i, "github.com/percona-platform/dbaas-controller")
				fmt.Fprintf(f, "\t%q -> %q;\n", p, i)
			}
		}
	}

	fmt.Fprintf(f, "}\n")
}
