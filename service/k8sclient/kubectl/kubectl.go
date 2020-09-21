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
	"strconv"
	"strings"

	"github.com/percona/pmm/utils/pdeathsig"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/percona-platform/dbaas-controller/utils/logger"
)

const (
	kubeconfigFileName      = "kubeconfig.json"
	defaultPmmServerKubectl = "/opt/dbaas-tools/bin/kubectl-1.16"
	defaultDevEnvKubectl    = "minikube kubectl --"
)

// KubeCtl wraps kubectl CLI with version selection and kubeconfig handling.
type KubeCtl struct {
	l      logger.Logger
	cmd    []string
	tmpDir string
}

// NewKubeCtl creates a new KubeCtl object with a given logger.
func NewKubeCtl(ctx context.Context, kubeconfig string) (*KubeCtl, error) {
	l := logger.Get(ctx)
	l = l.WithField("component", "kubectl")

	// Firstly lookup default kubectl to get Kubernetes Server version.
	defaultKubectl, err := getDefaultKubectlCmd()
	if err != nil {
		return nil, err
	}

	// Cannot identify k8s server version on non local env without kubeconfig (w/o address of k8s server).
	if kubeconfig == "" {
		return &KubeCtl{
			l:   l,
			cmd: defaultKubectl,
		}, nil
	}

	// Handle kubeconfig.
	tmpDir, kubeconfigPath, err := saveKubeconfig(kubeconfig)
	if err != nil {
		return nil, err
	}

	l.Infof("kubectl config: %q", kubeconfigPath)

	// Handle kubectl versions
	cmd, err := getKubectlCmd(ctx, defaultKubectl, kubeconfigPath)
	if err != nil {
		return nil, err
	}

	l.Infof("Using %q", strings.Join(cmd, " "))

	cmd = append(cmd, fmt.Sprintf("--kubeconfig=%s", kubeconfigPath))

	return &KubeCtl{
		l:      l,
		cmd:    cmd,
		tmpDir: tmpDir,
	}, nil
}

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

func getDefaultKubectlCmd() ([]string, error) {
	kubectlPath, errPmmServerKubectl := exec.LookPath(defaultPmmServerKubectl)
	if errPmmServerKubectl == nil {
		return []string{kubectlPath}, nil
	}

	// Assume it's local dev env.
	minikubeCmd := strings.Split(defaultDevEnvKubectl, " ")
	minikubePath, errDevEnvKubectl := exec.LookPath(minikubeCmd[0])
	if errDevEnvKubectl == nil {
		minikubeCmd[0] = minikubePath

		return minikubeCmd, nil
	}

	return nil, errors.Errorf("cannot find default kubectl:%s:%s", errPmmServerKubectl, errDevEnvKubectl)
}

// getKubectlCmd gets correct version of kubectl binary for Kubernetes cluster.
func getKubectlCmd(ctx context.Context, defaultKubectl []string, kubeconfigPath string) ([]string, error) {
	versionsJSON, err := getVersions(ctx, defaultKubectl, kubeconfigPath)
	if err != nil {
		return nil, err
	}

	kubectlCmdNames, err := selectCorrectKubectlVersions(versionsJSON)
	if err != nil {
		return nil, err
	}

	return lookupCorrectKubectlCmd(defaultKubectl, kubectlCmdNames)
}

func lookupCorrectKubectlCmd(defaultKubectl, kubectlCmdNames []string) ([]string, error) {
	for _, kubectlCmdName := range kubectlCmdNames {
		kubectlPath, err := exec.LookPath(kubectlCmdName)
		if err == nil {
			return []string{kubectlPath}, nil
		}
	}

	// if none found (pass empty kubectlCmdNames) use default version of kubectl.
	return defaultKubectl, nil
}

// getVersions gets kubectl and Kubernetes cluster version.
func getVersions(ctx context.Context, kubectlCmd []string, kubeconfigPath string) ([]byte, error) {
	versionsJSON, err := run(ctx, kubectlCmd, []string{"version", fmt.Sprintf("--kubeconfig=%s", kubeconfigPath), "-o", "json"}, nil)
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

	stdout, err := run(ctx, k.cmd, args, nil)
	if err != nil {
		return err
	}
	return json.Unmarshal(stdout, res)
}

// Apply executes `kubectl apply` with given resource.
func (k *KubeCtl) Apply(ctx context.Context, res meta.Object) error {
	_, err := run(ctx, k.cmd, []string{"apply", "-f", "-"}, res)
	return err
}

// Delete executes `kubectl delete` with given resource.
func (k *KubeCtl) Delete(ctx context.Context, res meta.Object) error {
	_, err := run(ctx, k.cmd, []string{"delete", "-f", "-"}, res)
	return err
}

// Run wraps func run.
func (k *KubeCtl) Run(ctx context.Context, args []string, stdin interface{}) ([]byte, error) {
	out, err := run(ctx, k.cmd, args, stdin)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// run executes kubectl with given kubectl binary/command, arguments and stdin data (encoded as JSON),
// and returns stdout, stderr and execution error.
func run(ctx context.Context, kubectlCmd []string, args []string, stdin interface{}) ([]byte, error) {
	l := logger.Get(ctx)
	l = l.WithField("component", "kubectl")
	args = append(kubectlCmd, args...)
	argsString := strings.Join(args, " ")

	var inBuf bytes.Buffer
	if stdin != nil {
		e := json.NewEncoder(&inBuf)
		e.SetIndent("", "  ")
		if err := e.Encode(stdin); err != nil {
			return nil, err
		}
		l.Debugf("Running %s with input:\n%s", argsString, inBuf.String())
	} else {
		l.Debugf("Running %s", argsString)
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

	l.Debug(outBuf.String())
	l.Debug(errBuf.String())
	return outBuf.Bytes(), err
}
