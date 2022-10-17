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
package kube

import (
	psmdbv1 "github.com/percona/percona-server-mongodb-operator/pkg/apis/psmdb/v1"
	pxcv1 "github.com/percona/percona-xtradb-cluster-operator/pkg/apis/pxc/v1"
)

type DBCluster struct {
	State          string
	Pause          bool
	Name           string
	CRImage        string
	ContainerNames []string
}

func NewDBClusterInfoFromPXC(cluster *pxcv1.PerconaXtraDBCluster) DBCluster {
	if cluster == nil || cluster.Spec.PXC == nil {
		return DBCluster{
			State: string(pxcv1.AppStateUnknown),
		}
	}
	return DBCluster{
		CRImage:        cluster.Spec.PXC.Image,
		State:          string(cluster.Status.Status),
		Pause:          cluster.Spec.Pause,
		Name:           cluster.Name,
		ContainerNames: []string{"pxc"},
	}
}

func NewDBClusterInfoFromPSMDB(cluster *psmdbv1.PerconaServerMongoDB) DBCluster {
	if cluster == nil || cluster == new(psmdbv1.PerconaServerMongoDB) || cluster.Status.State == "" {
		return DBCluster{
			State: string(pxcv1.AppStateUnknown),
		}
	}
	return DBCluster{
		CRImage:        cluster.Spec.Image,
		State:          string(cluster.Status.State),
		Pause:          cluster.Spec.Pause,
		Name:           cluster.Name,
		ContainerNames: []string{"pxc"},
	}
}
