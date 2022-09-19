package kube

import (
	psmdbv1 "github.com/percona/percona-server-mongodb-operator/pkg/apis/psmdb/v1"
	pxcv1 "github.com/percona/percona-xtradb-cluster-operator/pkg/apis/pxc/v1"
)

type DBCluster interface {
	State() pxcv1.AppState
	Pause() bool
	GetName() string
	CRImage() string
	SetImage(string)
}

type PXCCluster pxcv1.PerconaXtraDBCluster
type PSMDBCluster psmdbv1.PerconaServerMongoDB

func (c *PXCCluster) State() pxcv1.AppState {
	return c.Status.Status
}
func (c *PXCCluster) Pause() bool {
	return c.Spec.Pause
}
func (c *PXCCluster) GetName() string {
	return c.Name
}
func (c *PXCCluster) CRImage() string {
	return c.Spec.PXC.Image
}
func (c *PXCCluster) SetImage(img string) {
	c.Spec.PXC.Image = img
}
func (c *PXCCluster) Original() *pxcv1.PerconaXtraDBCluster {
	v := pxcv1.PerconaXtraDBCluster(*c)
	return &v
}
func (c *PSMDBCluster) State() pxcv1.AppState {
	return pxcv1.AppState(c.Status.State)
}
