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

package pxc

import (
	"github.com/percona-platform/dbaas-controller/service/k8sclient/internal/common"
)

// PerconaXtraDBClusterRestoreSpec defines the desired state of PerconaXtraDBClusterRestore.
type PerconaXtraDBClusterRestoreSpec struct {
	PXCCluster   string           `json:"pxcCluster"`
	BackupName   string           `json:"backupName"`
	BackupSource *PXCBackupStatus `json:"backupSource"`
}

// PerconaXtraDBClusterRestoreStatus defines the observed state of PerconaXtraDBClusterRestore.
type PerconaXtraDBClusterRestoreStatus struct {
	State    BcpRestoreStates `json:"state,omitempty"`
	Comments string           `json:"comments,omitempty"`
}

// PerconaXtraDBClusterRestore is the Schema for the perconaxtradbclusterrestores API.
type PerconaXtraDBClusterRestore struct {
	common.TypeMeta   // anonymous for embedding
	common.ObjectMeta `json:"metadata,omitempty"`

	Spec   PerconaXtraDBClusterRestoreSpec   `json:"spec,omitempty"`
	Status PerconaXtraDBClusterRestoreStatus `json:"status,omitempty"`
}

// PerconaXtraDBClusterRestoreList contains a list of PerconaXtraDBClusterRestore.
type PerconaXtraDBClusterRestoreList struct {
	common.TypeMeta // anonymous for embedding

	Items []PerconaXtraDBClusterRestore `json:"items"`
}

// BcpRestoreStates backup restore states.
type BcpRestoreStates string
