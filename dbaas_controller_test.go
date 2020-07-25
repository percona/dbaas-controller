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

package dbaascontroller

import (
	"bytes"
	"fmt"
	"go/build"
	"io/ioutil"
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
		imports := packageImports(t, path)

		for _, b := range c.blacklistPrefixes {
			for i := range imports {
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

		allImports[path] = imports
	}

	err := ioutil.WriteFile("packages.dot", graph(allImports), 0644) //nolint:gosec
	require.NoError(t, err)
}

// packageImports returns all packages imported by a given package (non-recursively).
func packageImports(t *testing.T, path string) map[string]struct{} {
	p, err := build.Import(path, ".", 0)
	require.NoError(t, err)

	res := make(map[string]struct{})

	for _, i := range p.Imports {
		res[i] = struct{}{}
	}
	for _, i := range p.TestImports {
		res[i] = struct{}{}
	}
	for _, i := range p.XTestImports {
		res[i] = struct{}{}
	}

	return res
}

// graph returns a Graphviz dot dependency graph for given imports
// with some nodes and edges removed for simplicity.
func graph(allImports map[string]map[string]struct{}) []byte {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "digraph packages {\n")

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
				fmt.Fprintf(&buf, "\t%q -> %q;\n", p, i)
			}
		}
	}

	fmt.Fprintf(&buf, "}\n")
	return buf.Bytes()
}
