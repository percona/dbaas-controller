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

// Package kubectl provides kubectl CLI wrapper.
package kubectl

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/avast/retry-go"
	"github.com/percona/pmm/utils/pdeathsig"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

var reservedProxyPorts = new(sync.Map)

// reserveProxyPort reserves a ramdom proxy port from range [10000, 19999).
// It stores the proxy command under the reserved port so we can get back to it later.
func (k *KubeCtl) reserveProxyPort(cmd *exec.Cmd) string {
	random := rand.New(rand.NewSource(time.Now().UnixNano()))
	var port int
	for {
		port = 10000 + random.Intn(10000)
		_, loaded := reservedProxyPorts.LoadOrStore(strconv.Itoa(port), cmd)
		if !loaded {
			break
		}
	}
	return strconv.Itoa(port)
}

// RunProxy runs kubectl proxy on port that is returned.
func (k *KubeCtl) RunProxy(ctx context.Context) (string, error) {
	var port string
	// Try to run kubectl proxy on random port.
	err := retry.Do(
		func() error {
			cmd := exec.CommandContext(ctx, k.cmd[0], k.cmd[1:]...) //nolint:gosec
			// Reserve a port so we don't try to use the same port from more
			// goroutines at the same time.
			port = k.reserveProxyPort(cmd)
			cmd.Args = append(cmd.Args, "proxy", "--port="+port)
			var err error
			defer func() {
				if err != nil {
					reservedProxyPorts.Delete(port)
					port = ""
				}
			}()

			// Prepare the command
			pdeathsig.Set(cmd, unix.SIGKILL)
			outBuf := new(bytes.Buffer)
			errBuf := new(bytes.Buffer)
			cmd.Stdout = outBuf
			cmd.Stderr = errBuf
			envs := os.Environ()
			for _, env := range envs {
				if strings.HasPrefix(env, "PATH=") {
					env = fmt.Sprintf("PATH=%s:%s", dbaasToolPath, os.Getenv("PATH"))
				}
				cmd.Env = append(cmd.Env, env)
			}

			// Start the command
			err = cmd.Start()
			if err != nil {
				if strings.Contains(errBuf.String(), "NotFound") {
					return ErrNotFound
				}
				return &kubeCtlError{
					err:    errors.WithStack(err),
					cmd:    strings.Join(cmd.Args, " "),
					stderr: errBuf.String(),
				}
			}

			// Wait for proxy to become alive.
			err = retry.Do(
				func() error {
					var conn net.Conn
					conn, err = net.DialTimeout("tcp", net.JoinHostPort("localhost", port), time.Second)
					if conn != nil {
						conn.Close() //nolint:errcheck,gosec
						return nil
					}
					return err
				},
				retry.Context(ctx),
			)
			if err != nil {
				k.StopProxy(port)
			}
			return errors.Wrap(err, "failed to reach Kubernetes API")
		},
		retry.Context(ctx),
	)
	return port, errors.Wrap(err, "failed to run kubectl proxy")
}

// StopProxy stops kubectl proxy if there is any running on the given port
// and then releases the port for use by future proxy processes.
func (k *KubeCtl) StopProxy(port string) error {
	defer reservedProxyPorts.Delete(port)
	cmd, ok := reservedProxyPorts.Load(port)
	if !ok {
		return errors.Errorf("trying to release proxy port %s that is not reserved by any proxy process", port)
	}
	err := cmd.(*exec.Cmd).Process.Kill()
	if err != nil {
		return err
	}
	err = cmd.(*exec.Cmd).Wait()
	if !strings.Contains(err.Error(), "killed") {
		return err
	}
	return nil
}
