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

// Package kubectl provides kubectl client.
package kubectl

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"sync"

	pxc "github.com/percona/percona-xtradb-cluster-operator/pkg/apis/pxc/v1"
	"github.com/pkg/errors"

	"github.com/percona-platform/dbaas-controller/logger"
)

// ErrNotFound is an error in case of object not found.
var ErrNotFound = errors.New("object not found")

// NewKubeCtl returns new KubeCtl object.
func NewKubeCtl(l logger.Logger) *KubeCtl {
	return &KubeCtl{
		l: l.WithField("component", "kubectl"),
	}
}

// KubeCtl is a wrapper for kubectl cli command.
type KubeCtl struct {
	l   logger.Logger
	cmd []string

	once sync.Once
}

// run runs kubectl commands in cli.
func (k *KubeCtl) run(ctx context.Context, args []string, stdin interface{}) ([]byte, []byte, error) {
	k.once.Do(k.init)
	var stdOutBuf bytes.Buffer
	var stdErrBuf bytes.Buffer
	args = append(k.cmd, args...)

	var buf bytes.Buffer
	e := json.NewEncoder(&buf)
	if err := e.Encode(stdin); err != nil {
		return nil, nil, err
	}
	k.l.Debugf("%v : %s", args, buf.String())

	cmd := exec.CommandContext(ctx, args[0], args[1:]...) // nolint:gosec
	cmd.Stdin = &buf
	cmd.Stdout = &stdOutBuf
	cmd.Stderr = &stdErrBuf
	defer func() {
		k.l.Debugf("%s\n%s", stdOutBuf.String(), stdErrBuf.String())
	}()
	if err := cmd.Run(); err != nil {
		return stdOutBuf.Bytes(), stdErrBuf.Bytes(), err
	}
	return stdOutBuf.Bytes(), stdErrBuf.Bytes(), nil
}

// Apply runs kubectl apply command.
func (k *KubeCtl) Apply(ctx context.Context, res *pxc.PerconaXtraDBCluster) error {
	_, _, err := k.run(ctx, []string{"apply", "-f", "-"}, res)
	if err != nil {
		return err
	}
	return nil
}

// Delete runs kubectl delete command.
func (k *KubeCtl) Delete(ctx context.Context, res *pxc.PerconaXtraDBCluster) error {
	_, _, err := k.run(ctx, []string{"delete", "-f", "-"}, res)
	if err != nil {
		return err
	}
	return nil
}

// Get runs kubectl get command and returns `ErrNotFound` if object not found.
func (k *KubeCtl) Get(ctx context.Context, kind string, name string) ([]byte, error) {
	args := []string{"get", "-o=json", kind}
	if name != "" {
		args = append(args, name)
	}
	stdout, stderr, err := k.run(ctx, args, nil)
	if err != nil {
		if strings.Contains(string(stderr), "not found") {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return stdout, nil
}

func (k *KubeCtl) init() {
	var cmd = []string{"kubectl"}

	path, err := exec.LookPath("kubectl")
	k.l.Debugf("path: %s, err: %v", path, err)
	if e, ok := err.(*exec.Error); err != nil && ok && e.Err == exec.ErrNotFound {
		cmd = []string{"minikube", "kubectl", "--"}
	}
	k.cmd = cmd
}
