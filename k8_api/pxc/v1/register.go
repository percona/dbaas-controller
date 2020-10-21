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

// NOTE: Boilerplate only.  Ignore this file.

// Package v1 contains API Schema definitions for the pxc v1 API group
// +k8s:deepcopy-gen=package,register
// +groupName=pxc.percona.com
package v1

import (
	"strings"

	"github.com/percona/percona-xtradb-cluster-operator/version"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

//nolint:gochecknoglobals
var (
	mainSchemeGroupVersion = schema.GroupVersion{Group: "pxc.percona.com", Version: strings.ReplaceAll("v"+version.Version, ".", "-")}
	MainSchemeBuilder      = scheme.Builder{GroupVersion: mainSchemeGroupVersion}
	// SchemeGroupVersion is group version used to register these objects.
	SchemeGroupVersion = schema.GroupVersion{Group: "pxc.percona.com", Version: "v1"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme.
	SchemeBuilder = &scheme.Builder{GroupVersion: SchemeGroupVersion}
)

//nolint:gochecknoinits
func init() {
	SchemeBuilder.Register(
		new(PerconaXtraDBClusterBackup), new(PerconaXtraDBClusterBackupList),
	)
	MainSchemeBuilder.Register(new(PerconaXtraDBCluster), new(PerconaXtraDBClusterList))
}
