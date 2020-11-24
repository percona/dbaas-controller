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

package v1

import (
	meta "github.com/percona-platform/dbaas-controller/k8s_api/meta/v1"
)

// PerconaXtraDBClusterRestoreSpec defines the desired state of PerconaXtraDBClusterRestore.
type PerconaXtraDBClusterRestoreSpec struct {
	PXCCluster   string           `json:"pxcCluster"`
	BackupName   string           `json:"backupName"`
	BackupSource *PXCBackupStatus `json:"backupSource"`
}

// PerconaXtraDBClusterRestoreStatus defines the observed state of PerconaXtraDBClusterRestore.
type PerconaXtraDBClusterRestoreStatus struct {
	State         BcpRestoreStates `json:"state,omitempty"`
	Comments      string           `json:"comments,omitempty"`
	CompletedAt   *meta.Time       `json:"completed,omitempty"`
	LastScheduled *meta.Time       `json:"lastscheduled,omitempty"`
}

// PerconaXtraDBClusterRestore is the Schema for the perconaxtradbclusterrestores API.
type PerconaXtraDBClusterRestore struct {
	meta.TypeMeta   `json:",inline"`
	meta.ObjectMeta `json:"metadata,omitempty"`

	Spec   PerconaXtraDBClusterRestoreSpec   `json:"spec,omitempty"`
	Status PerconaXtraDBClusterRestoreStatus `json:"status,omitempty"`
}

// PerconaXtraDBClusterRestoreList contains a list of PerconaXtraDBClusterRestore.
type PerconaXtraDBClusterRestoreList struct {
	meta.TypeMeta `json:",inline"`
	meta.ListMeta `json:"metadata,omitempty"`
	Items         []PerconaXtraDBClusterRestore `json:"items"`
}

// BcpRestoreStates backup restore states.
type BcpRestoreStates string
