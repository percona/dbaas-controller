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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/percona/pmm/utils/pdeathsig"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/percona-platform/dbaas-controller/utils/logger"
)

// KubeCtl wraps kubectl CLI with version selection and kubeconfig handling.
type KubeCtl struct {
	l      logger.Logger
	cmd    []string
	tmpDir string
}

const kubeconfigFileName = "kubeconfig.json"

// NewKubeCtl creates a new KubeCtl object with a given logger.
func NewKubeCtl(l logger.Logger, kubeconfig string) (*KubeCtl, error) {
	// TODO Handle kubectl versions https://jira.percona.com/browse/PMM-6348

	cmd := []string{"dbaas-kubectl-1.16"}
	kubectlPath, err := exec.LookPath(cmd[0])
	l.Debugf("kubectlPath: %s, err: %v", kubectlPath, err)
	cmd = []string{kubectlPath}
	if e, ok := err.(*exec.Error); err != nil && ok && e.Err == exec.ErrNotFound {
		cmd = []string{"minikube", "kubectl", "--"}
	}

	// Handle kubeconfig.
	tmpDir, kubeconfigPath, err := saveKubeconfig(kubeconfig)
	if err != nil {
		l.Debugf("Cannot save kubeconfig: %s", err)
		return nil, err
	}

	cmd = append(cmd, fmt.Sprintf("--kubeconfig=%s", kubeconfigPath))
	l.Debugf("Using %q", strings.Join(cmd, " "))

	return &KubeCtl{
		l:      l.WithField("component", "kubectl"),
		cmd:    cmd,
		tmpDir: tmpDir,
	}, nil
}

// saveKubeconfig handles kubeconfig.
func saveKubeconfig(kubeconfig string) (string, string, error) {
	tmpDir, err := ioutil.TempDir("", "dbaas-controller-kubeconfigs-")
	if err != nil {
		return "", "", err
	}

	kubeconfigPath := path.Join(tmpDir, kubeconfigFileName)

	err = ioutil.WriteFile(kubeconfigPath, []byte(kubeconfig), 0o600)
	if err != nil {
		return "", "", err
	}

	return tmpDir, kubeconfigPath, nil
}

// Cleanup removes temporary files created by that object.
func (k *KubeCtl) Cleanup() error {
	return os.RemoveAll(k.tmpDir)
}

// Get executes `kubectl get` with given object kind and optional name,
// and decodes resource into `res`.
func (k *KubeCtl) Get(ctx context.Context, kind string, name string, res interface{}) error {
	args := []string{"get", "-o=json", kind}
	if name != "" {
		args = append(args, name)
	}

	stdout, err := k.run(ctx, args, nil)
	if err != nil {
		return err
	}

	return json.Unmarshal(stdout, res)
}

// Apply executes `kubectl apply` with given resource.
func (k *KubeCtl) Apply(ctx context.Context, res meta.Object) error {
	_, err := k.run(ctx, []string{"apply", "-f", "-"}, res)
	return err
}

// Delete executes `kubectl delete` with given resource.
func (k *KubeCtl) Delete(ctx context.Context, res meta.Object) error {
	_, err := k.run(ctx, []string{"delete", "-f", "-"}, res)
	return err
}

// run executes kubectl with given arguments and stdin data (encoded as JSON),
// and returns stdout, stderr and execution error.
func (k *KubeCtl) run(ctx context.Context, args []string, stdin interface{}) ([]byte, error) {
	args = append(k.cmd, args...)
	argsString := strings.Join(args, " ")

	var inBuf bytes.Buffer
	if stdin != nil {
		e := json.NewEncoder(&inBuf)
		e.SetIndent("", "  ")
		if err := e.Encode(stdin); err != nil {
			return nil, err
		}
		k.l.Debugf("Running %s with input:\n%s", argsString, inBuf.String())
	} else {
		k.l.Debugf("Running %s", argsString)
	}

	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	cmd := exec.CommandContext(ctx, args[0], args[1:]...) //nolint:gosec
	pdeathsig.Set(cmd, unix.SIGKILL)
	cmd.Stdin = &inBuf
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if err != nil {
		err = &kubeCtlError{
			err:    errors.WithStack(err),
			cmd:    argsString,
			stderr: errBuf.String(),
		}
	}

	k.l.Debug(outBuf.String())
	k.l.Debug(errBuf.String())
	return outBuf.Bytes(), err
}
