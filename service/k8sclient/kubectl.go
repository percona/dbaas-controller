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

	"github.com/percona/pmm/utils/pdeathsig"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/percona-platform/dbaas-controller/logger"
)

// errNotFound is an error in case of object not found.
var errNotFound = errors.New("object not found")

// kubeCtl wraps kubectl CLI with version selection and kubeconfig handling.
type kubeCtl struct {
	l   logger.Logger
	cmd []string
}

// newKubeCtl creates a new kubeCtl object with a given logger.
func newKubeCtl(l logger.Logger) *kubeCtl {
	// TODO accept and handle version
	// TODO find correct kubectl binary for given version
	// TODO accept and handle kubeconfig

	cmd := []string{"dbaas-kubectl-1.16"}
	path, err := exec.LookPath(cmd[0])
	l.Debugf("path: %s, err: %v", path, err)
	if e, ok := err.(*exec.Error); err != nil && ok && e.Err == exec.ErrNotFound {
		cmd = []string{"minikube", "kubectl", "--"}
	}
	l.Debugf("Using %q", strings.Join(cmd, " "))

	return &kubeCtl{
		l:   l.WithField("component", "kubectl"),
		cmd: cmd,
	}
}

// run executes kubectl with given arguments and stdin data (encoded as JSON),
// and returns stdout, stderr and execution error.
func (k *kubeCtl) run(ctx context.Context, args []string, stdin interface{}) ([]byte, []byte, error) {
	args = append(k.cmd, args...)

	var inBuf bytes.Buffer
	if stdin != nil {
		e := json.NewEncoder(&inBuf)
		e.SetIndent("", "  ")
		if err := e.Encode(stdin); err != nil {
			return nil, nil, err
		}
		k.l.Debugf("Running %s with input:\n%s", strings.Join(args, " "), inBuf.String())
	} else {
		k.l.Debugf("Running %s", strings.Join(args, " "))
	}

	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	cmd := exec.CommandContext(ctx, args[0], args[1:]...) //nolint:gosec
	pdeathsig.Set(cmd, unix.SIGKILL)
	cmd.Stdin = &inBuf
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()

	k.l.Debugf(outBuf.String()) // FIXME
	k.l.Debugf(errBuf.String())
	return outBuf.Bytes(), errBuf.Bytes(), err
}

// apply executes `kubectl apply` with given resource.
func (k *kubeCtl) apply(ctx context.Context, res meta.Object) error {
	_, _, err := k.run(ctx, []string{"apply", "-f", "-"}, res)
	return err
}

// delete executes `kubectl delete` with given resource.
func (k *kubeCtl) delete(ctx context.Context, res meta.Object) error {
	_, _, err := k.run(ctx, []string{"delete", "-f", "-"}, res)
	return err
}

// get executes `kubectl get` with given object kind and optional name,
// and decodes resource into `res`.
// It returns `errNotFound` if object is not found.
func (k *kubeCtl) get(ctx context.Context, kind string, name string, res interface{}) error {
	args := []string{"get", "-o=json", kind}
	if name != "" {
		args = append(args, name)
	}

	stdout, stderr, err := k.run(ctx, args, nil)
	if err != nil {
		if strings.Contains(string(stderr), "not found") { // FIXME
			return errNotFound
		}
		return err
	}

	return json.Unmarshal(stdout, res)
}
