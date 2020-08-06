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
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/percona/pmm/utils/pdeathsig"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/percona-platform/dbaas-controller/utils/logger"
)

const (
	defaultKubectl = "dbaas-kubectl-1.16"
	devKubectl     = "minikube kubectl --"
)

// KubeCtl wraps kubectl CLI with version selection and kubeconfig handling.
type KubeCtl struct {
	l   logger.Logger
	cmd []string
}

// NewKubeCtl creates a new KubeCtl object with a given logger.
func NewKubeCtl(l logger.Logger) *KubeCtl {
	// Handle kubectl versions
	cmd, err := getKubectlCmd()
	if err != nil {
		l.Debugf("Cannot find kubectl binary: %s", err)
		return nil
	}
	l.Debugf("Using %q", strings.Join(cmd, " "))

	// TODO Handle kubeconfig https://jira.percona.com/browse/PMM-6347

	return &KubeCtl{
		l:   l.WithField("component", "kubectl"),
		cmd: cmd,
	}
}

// getKubectlCmd gets correct version of kubectl binary for Kubernetes cluster.
func getKubectlCmd() ([]string, error) {
	// Firstly lookup default kubectl to get Kubernetes Server version.
	kubectlCmd, err := lookupCorrectKubectlCmd([]string{defaultKubectl})
	if err != nil {
		return nil, err
	}

	versionsJSON, err := getVersions(kubectlCmd)
	if err != nil {
		return nil, err
	}

	kubectlCmdNames, err := selectCorrectKubectlVersions(versionsJSON)
	if err != nil {
		return nil, err
	}

	return lookupCorrectKubectlCmd(kubectlCmdNames)
}

// getVersions gets kubectl and Kubernetes cluster version.
func getVersions(kubectlCmd []string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	getVersionCmd := append(kubectlCmd, "version", "-o", "json")
	cmd := exec.CommandContext(ctx, getVersionCmd[0], getVersionCmd[1:]...) //nolint:gosec
	versionsJSON, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}
	return versionsJSON, nil
}

// selectCorrectKubectlVersions select list correct versions of kubectl binary for Kubernetes cluster.
//
// https://kubernetes.io/docs/setup/release/version-skew-policy/#kubectl
// > kubectl is supported within one minor version (older or newer) of kube-apiserver.
// > Example:
// > 	kube-apiserver is at 1.18
// > 	kubectl is supported at 1.19, 1.18, and 1.17.
func selectCorrectKubectlVersions(versionsJSON []byte) ([]string, error) {
	var kubectlCmdNames []string
	ver := struct {
		ServerVersion struct {
			Major string `json:"major"`
			Minor string `json:"minor"`
		} `json:"serverVersion"`
	}{}

	if err := json.Unmarshal(versionsJSON, &ver); err != nil {
		return nil, err
	}

	serverMajor, err := strconv.Atoi(ver.ServerVersion.Major)
	if err != nil {
		return nil, err
	}

	serverMinor, err := strconv.Atoi(ver.ServerVersion.Minor)
	if err != nil {
		return nil, err
	}

	// Iterate from newer to older version. Append default as the last.
	for minor := serverMinor + 1; minor >= serverMinor-1; minor-- {
		kubectlCmdNames = append(kubectlCmdNames, fmt.Sprintf("kubectl-%d.%d", serverMajor, minor))
	}
	return kubectlCmdNames, nil
}

func lookupCorrectKubectlCmd(kubectlCmdNames []string) ([]string, error) {
	for _, kubectlCmdName := range kubectlCmdNames {
		kubectlPath, err := exec.LookPath(kubectlCmdName)
		if err == nil {
			return []string{kubectlPath}, nil
		}
	}

	// if none found (pass empty kubectlCmdNames) use dev version of kubectl.
	return strings.Split(devKubectl, " "), nil
}

// Cleanup removes temporary files created by that object.
func (k *KubeCtl) Cleanup() {
	// TODO Remove kubeconfig file https://jira.percona.com/browse/PMM-6347
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
