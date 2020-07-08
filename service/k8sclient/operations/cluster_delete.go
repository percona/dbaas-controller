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

package operations

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	pxc "github.com/percona/percona-xtradb-cluster-operator/pkg/apis/pxc/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/percona-platform/dbaas-controller/logger"
	"github.com/percona-platform/dbaas-controller/service/k8sclient/kubectl"
)

// NewClusterDelete returns new object of ClusterDelete.
func NewClusterDelete(l logger.Logger, name string, size int32) *ClusterDelete {
	return &ClusterDelete{
		kubectl:  kubectl.NewKubeCtl(l),
		l:        l,
		name:     name,
		size:     size,
		pxcImage: "percona/percona-xtradb-cluster-operator:1.4.0-pxc8.0",
		kind:     "PerconaXtraDBCluster",
	}
}

// ClusterDelete deletes kubernetes cluster.
type ClusterDelete struct {
	kubectl *kubectl.KubeCtl
	l       logger.Logger

	name     string
	size     int32
	pxcImage string
	kind     string
}

// Start starts cluster deleting process.
func (c *ClusterDelete) Start(ctx context.Context) error {
	res := &pxc.PerconaXtraDBCluster{
		TypeMeta: meta.TypeMeta{
			APIVersion: "pxc.percona.com/v1-4-0",
			Kind:       c.kind,
		},
		ObjectMeta: meta.ObjectMeta{
			Name: c.name,
		},
	}
	return c.delete(ctx, res)
}

// Wait waits until cluster is fully deleted.
func (c *ClusterDelete) Wait(ctx context.Context) error {
	res, err := c.kubectl.Wait(ctx, c.kind, c.name, "delete", time.Minute)
	c.l.Infof("%s", string(res))
	// TODO wait until pods are terminated.
	// TODO: fail after timeout.
	if err != nil {
		if strings.Contains(string(res), "not found") {
			return nil
		}

		for {
			res, err := c.get(ctx, c.kind, c.name)
			c.l.Infof("%s", string(res))
			if err != nil {
				if strings.Contains(string(res), "not found") {
					return nil
				}
				c.l.Errorf("%v", err)
			}
			time.Sleep(30 * time.Second)
		}
	}
	return nil
}

func (c *ClusterDelete) get(ctx context.Context, kind, name string) ([]byte, error) {
	return c.kubectl.Run(ctx, []string{"get", "-o=json", kind, name}, nil)
}

func (c *ClusterDelete) delete(ctx context.Context, res *pxc.PerconaXtraDBCluster) error {
	var buf bytes.Buffer
	e := json.NewEncoder(&buf)
	e.SetIndent("", "  ")
	if err := e.Encode(res); err != nil {
		log.Fatal(err)
	}
	log.Printf("apply:\n%s", buf.String())

	b, err := c.kubectl.Run(ctx, []string{"delete", "-f", "-"}, &buf)
	if err != nil {
		return err
	}
	log.Printf("%s", b)
	return nil
}
