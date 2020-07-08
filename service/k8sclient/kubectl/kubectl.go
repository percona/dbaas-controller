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

package kubectl

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"time"

	"github.com/percona-platform/dbaas-controller/logger"
)

var cmd = []string{"minikube", "kubectl", "--"} //nolint:gochecknoglobals

// NewKubeCtl returns new KubeCtl object.
func NewKubeCtl(l logger.Logger) *KubeCtl {
	return &KubeCtl{l: l.WithField("component", "kubectl")}
}

// KubeCtl is a wrapper for kubectl cli command.
type KubeCtl struct {
	l logger.Logger
}

// Run runs kubectl commands in cli.
func (k *KubeCtl) Run(ctx context.Context, args []string, stdin io.Reader) ([]byte, error) {
	var buf bytes.Buffer
	var errBuf bytes.Buffer
	args = append(cmd, args...)
	k.l.Debugf("%v", args)
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Stdin = stdin
	cmd.Stdout = &buf
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return errBuf.Bytes(), err
	}
	return buf.Bytes(), nil
}

// Wait runs kubectl wait command in cli.
func (k *KubeCtl) Wait(ctx context.Context, kind, name, condition string, timeout time.Duration) ([]byte, error) {
	res, err := k.Run(ctx, []string{"wait", "--for=" + condition, "--timeout=" + timeout.String(), kind, name}, nil)
	return res, err
}
