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

package k8sclient

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"sync"

	"github.com/pkg/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/percona-platform/dbaas-controller/logger"
)

// errNotFound is an error in case of object not found.
var errNotFound = errors.New("object not found")

// newKubeCtl returns new kubeCtl object.
func newKubeCtl(l logger.Logger) *kubeCtl {
	return &kubeCtl{
		l: l.WithField("component", "kubectl"),
	}
}

// kubeCtl is a wrapper for kubectl cli command.
type kubeCtl struct {
	l   logger.Logger
	cmd []string

	once sync.Once
}

// run runs kubectl commands in cli.
func (k *kubeCtl) run(ctx context.Context, args []string, stdin interface{}) ([]byte, []byte, error) {
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
		k.l.Debugf(stdOutBuf.String())
		k.l.Debugf(stdErrBuf.String())
	}()
	if err := cmd.Run(); err != nil {
		return stdOutBuf.Bytes(), stdErrBuf.Bytes(), err
	}
	return stdOutBuf.Bytes(), stdErrBuf.Bytes(), nil
}

// apply runs kubectl apply command.
func (k *kubeCtl) apply(ctx context.Context, res meta.Object) error {
	_, _, err := k.run(ctx, []string{"apply", "-f", "-"}, res)
	if err != nil {
		return err
	}
	return nil
}

// delete runs kubectl delete command.
func (k *kubeCtl) delete(ctx context.Context, res meta.Object) error {
	_, _, err := k.run(ctx, []string{"delete", "-f", "-"}, res)
	if err != nil {
		return err
	}
	return nil
}

// get runs kubectl get command and returns `errNotFound` if object not found.
func (k *kubeCtl) get(ctx context.Context, kind string, name string, res interface{}) error {
	args := []string{"get", "-o=json", kind}
	if name != "" {
		args = append(args, name)
	}
	stdout, stderr, err := k.run(ctx, args, nil)
	if err != nil {
		if strings.Contains(string(stderr), "not found") {
			return errNotFound
		}
		return err
	}

	return json.Unmarshal(stdout, res)
}

// init selects available kubectl in current system.
func (k *kubeCtl) init() {
	cmd := []string{"dbaas-kubectl-1.16"}

	path, err := exec.LookPath("dbaas-kubectl-1.16")
	k.l.Debugf("path: %s, err: %v", path, err)
	if e, ok := err.(*exec.Error); err != nil && ok && e.Err == exec.ErrNotFound {
		cmd = []string{"minikube", "kubectl", "--"}
	}
	k.cmd = cmd
}
