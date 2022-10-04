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
	return DBCluster{
		CRImage:        cluster.Spec.PXC.Image,
		State:          string(cluster.Status.Status),
		Pause:          cluster.Spec.Pause,
		Name:           cluster.Name,
		ContainerNames: []string{"pxc"},
	}
}
func NewDBClusterInfoFromPSMDB(cluster *psmdbv1.PerconaServerMongoDB) DBCluster {
	return DBCluster{
		CRImage:        cluster.Spec.Image,
		State:          string(cluster.Status.State),
		Pause:          cluster.Spec.Pause,
		Name:           cluster.Name,
		ContainerNames: []string{"pxc"},
	}
}
