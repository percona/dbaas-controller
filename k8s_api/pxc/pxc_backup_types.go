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
	"github.com/percona-platform/dbaas-controller/k8s_api/common"
)

// PerconaXtraDBClusterBackupList holds exported fields representing Percona XtraDB cluster backup list.
type PerconaXtraDBClusterBackupList struct {
	common.TypeMeta // anonymous for embedding

	Items []PerconaXtraDBClusterBackup `json:"items"`
}

// PerconaXtraDBClusterBackup represents a Percona XtraDB cluster backup.
type PerconaXtraDBClusterBackup struct {
	common.TypeMeta   // anonymous for embedding
	common.ObjectMeta `json:"metadata"`
	Spec              PXCBackupSpec   `json:"spec"`
	Status            PXCBackupStatus `json:"status,omitempty"`
	SchedulerName     string          `json:"schedulerName,omitempty"`
	PriorityClassName string          `json:"priorityClassName,omitempty"`
}

// PXCBackupSpec represents a PXC backup.
type PXCBackupSpec struct {
	PXCCluster  string `json:"pxcCluster"`
	StorageName string `json:"storageName,omitempty"`
}

// PXCBackupStatus PXC backup status.
type PXCBackupStatus struct {
	State       PXCBackupState       `json:"state,omitempty"`
	Destination string               `json:"destination,omitempty"`
	StorageName string               `json:"storageName,omitempty"`
	S3          *BackupStorageS3Spec `json:"s3,omitempty"`
}

// PXCBackupState PXC backup state string.
type PXCBackupState string
