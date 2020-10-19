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
	"errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PerconaXtraDBClusterRestoreSpec defines the desired state of PerconaXtraDBClusterRestore
type PerconaXtraDBClusterRestoreSpec struct {
	PXCCluster   string           `json:"pxcCluster"`
	BackupName   string           `json:"backupName"`
	BackupSource *PXCBackupStatus `json:"backupSource"`
}

// PerconaXtraDBClusterRestoreStatus defines the observed state of PerconaXtraDBClusterRestore
type PerconaXtraDBClusterRestoreStatus struct {
	State         BcpRestoreStates `json:"state,omitempty"`
	Comments      string           `json:"comments,omitempty"`
	CompletedAt   *metav1.Time     `json:"completed,omitempty"`
	LastScheduled *metav1.Time     `json:"lastscheduled,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PerconaXtraDBClusterRestore is the Schema for the perconaxtradbclusterrestores API
// +k8s:openapi-gen=true
type PerconaXtraDBClusterRestore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PerconaXtraDBClusterRestoreSpec   `json:"spec,omitempty"`
	Status PerconaXtraDBClusterRestoreStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PerconaXtraDBClusterRestoreList contains a list of PerconaXtraDBClusterRestore
type PerconaXtraDBClusterRestoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PerconaXtraDBClusterRestore `json:"items"`
}

type BcpRestoreStates string

const (
	RestoreNew          BcpRestoreStates = ""
	RestoreStarting                      = "Starting"
	RestoreStopCluster                   = "Stopping Cluster"
	RestoreRestore                       = "Restoring"
	RestoreStartCluster                  = "Starting Cluster"
	RestoreFailed                        = "Failed"
	RestoreSucceeded                     = "Succeeded"
)

func (cr *PerconaXtraDBClusterRestore) CheckNsetDefaults() error {
	if cr.Spec.PXCCluster == "" {
		return errors.New("pxcCluster can't be empty")
	}

	if cr.Spec.BackupName == "" && cr.Spec.BackupSource == nil {
		return errors.New("backupName and BackupSource can't be empty simultaneously")
	}

	return nil
}

func init() {
	SchemeBuilder.Register(&PerconaXtraDBClusterRestore{}, &PerconaXtraDBClusterRestoreList{})
}
