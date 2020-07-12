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
	"errors"
	"os/exec"
	"strings"

	pxc "github.com/percona/percona-xtradb-cluster-operator/pkg/apis/pxc/v1"

	"github.com/percona-platform/dbaas-controller/logger"
)

var ErrNotFound = errors.New("object not found")

// NewKubeCtl returns new KubeCtl object.
func NewKubeCtl(l logger.Logger) *KubeCtl {
	var cmd = []string{"minikube", "kubectl", "--"}

	return &KubeCtl{
		l:   l.WithField("component", "kubectl"),
		cmd: cmd, // TODO: select kubectl
	}
}

// KubeCtl is a wrapper for kubectl cli command.
type KubeCtl struct {
	l   logger.Logger
	cmd []string
}

// run runs kubectl commands in cli.
func (k *KubeCtl) run(ctx context.Context, args []string, stdin interface{}) ([]byte, []byte, error) {
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
	_, _, err := k.run(ctx, []string{"Delete", "-f", "-"}, res)
	if err != nil {
		return err
	}
	return nil
}

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
		k.l.Infof("%s", stderr)
		return nil, err
	}
	return stdout, nil
}
