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
	"strconv"
	"strings"

	"github.com/percona/pmm/utils/pdeathsig"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"

	"github.com/percona-platform/dbaas-controller/utils/logger"
)

const (
	dbaasToolPath           = "/opt/dbaas-tools/bin"
	defaultPmmServerKubectl = dbaasToolPath + "/kubectl-1.16"
	defaultDevEnvKubectl    = "minikube kubectl --"
)

// PatchType tells what kind of patch we want to perform.
// See https://kubernetes.io/docs/tasks/manage-kubernetes-objects/update-api-object-kubectl-patch/.
type PatchType string

const (
	// PatchTypeStrategic patches based on it's tags defined. Some are replaced, some are extended.
	PatchTypeStrategic PatchType = "strategic"
	// PatchTypeMerge indicates we want to replace entire parts of resource.
	PatchTypeMerge PatchType = "merge"
	// PatchTypeJSON is a series of operations representing the patch. See https://erosb.github.io/post/json-patch-vs-merge-patch/.
	PatchTypeJSON PatchType = "json"
)

// KubeCtl wraps kubectl CLI with version selection and kubeconfig handling.
type KubeCtl struct {
	l              logger.Logger
	cmd            []string
	kubeconfigPath string
}

// NewKubeCtl creates a new KubeCtl object with a given logger.
func NewKubeCtl(ctx context.Context, kubeconfig string) (*KubeCtl, error) {
	l := logger.Get(ctx)
	l = l.WithField("component", "kubectl")

	// Firstly lookup default kubectl to get Kubernetes Server version.
	defKubectls := []string{defaultPmmServerKubectl, defaultDevEnvKubectl}
	defaultKubectl, err := lookupCorrectKubectlCmd(nil, defKubectls)
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
	kubeconfigPath, err := saveKubeconfig(kubeconfig)
	if err != nil {
		return nil, err
	}

	l.Infof("kubectl config: %q", kubeconfigPath)

	k := &KubeCtl{
		l:              l,
		kubeconfigPath: kubeconfigPath,
	}

	// Handle kubectl versions
	cmd, err := getKubectlCmd(ctx, defaultKubectl, kubeconfigPath)
	if err != nil {
		e := k.Cleanup()
		if e != nil {
			l.Error(e)
		}
		return k, err // we return k only for tests
	}

	l.Infof("Using %q", strings.Join(cmd, " "))

	cmd = append(cmd, fmt.Sprintf("--kubeconfig=%s", kubeconfigPath))
	k.cmd = cmd
	return k, nil
}

func saveKubeconfig(kubeconfig string) (string, error) {
	tmpFile, err := ioutil.TempFile("", "dbaas-controller-kubeconfig-*")
	if err != nil {
		return "", err
	}

	_, err = tmpFile.Write([]byte(kubeconfig))
	if err != nil {
		_ = os.RemoveAll(tmpFile.Name())
		return "", err
	}

	if err := tmpFile.Close(); err != nil {
		_ = os.RemoveAll(tmpFile.Name())
		return "", err
	}

	return tmpFile.Name(), nil
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
		cmd := strings.Split(kubectlCmdName, " ")
		kubectlPath, err := exec.LookPath(cmd[0])
		if err == nil {
			return append([]string{kubectlPath}, cmd[1:]...), nil
		}
	}

	if defaultKubectl == nil {
		return nil, errors.Errorf("cannot find kubectl: %v, %v", defaultKubectl, kubectlCmdNames)
	}

	// if none found and default is not empty use default version of kubectl.
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

	serverMinor, err := strconv.Atoi(strings.TrimSuffix(ver.ServerVersion.Minor, "+")) // EKS is returning "serverVersion": { "major": "1", "minor": "16+" }
	if err != nil {
		return nil, err
	}

	// Iterate from newer to older version. Append default as the last.
	for minor := serverMinor + 1; minor >= serverMinor-1; minor-- {
		kubectlCmdNames = append(kubectlCmdNames, fmt.Sprintf("%s/kubectl-%d.%d", dbaasToolPath, serverMajor, minor))
	}
	return kubectlCmdNames, nil
}

// Cleanup removes temporary files created by that object.
func (k *KubeCtl) Cleanup() error {
	return os.RemoveAll(k.kubeconfigPath)
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
func (k *KubeCtl) Apply(ctx context.Context, res interface{}) error {
	_, err := run(ctx, k.cmd, []string{"apply", "-f", "-"}, res)
	return err
}

// Patch executes `kubectl patch` on given resource.
func (k *KubeCtl) Patch(ctx context.Context, patchType PatchType, resourceType, resourceName string, res interface{}) error {
	patch, err := json.Marshal(res)
	if err != nil {
		return err
	}
	if patchType == "" {
		patchType = PatchTypeStrategic
	}
	_, err = run(ctx, k.cmd, []string{"patch", resourceType, resourceName, "--type", string(patchType), "--patch", string(patch)}, nil)
	return err
}

// Delete executes `kubectl delete` with given resource.
func (k *KubeCtl) Delete(ctx context.Context, res interface{}) error {
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
	cmds := make([]string, len(kubectlCmd))
	copy(cmds, kubectlCmd)
	args = append(cmds, args...)
	argsString := strings.Join(args, " ")

	var inBuf bytes.Buffer
	if stdin != nil {
		if b, ok := stdin.([]byte); ok {
			inBuf.Write(b)
		} else {
			e := json.NewEncoder(&inBuf)
			e.SetIndent("", "  ")
			if err := e.Encode(stdin); err != nil {
				return nil, err
			}
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
	envs := os.Environ()
	for _, env := range envs {
		if strings.HasPrefix(env, "PATH=") {
			env = fmt.Sprintf("PATH=%s:%s", dbaasToolPath, os.Getenv("PATH"))
		}
		cmd.Env = append(cmd.Env, env)
	}
	err := cmd.Run()
	errOutput := errBuf.String()
	if err != nil {
		if strings.Contains(errOutput, "NotFound") {
			l.Warn(errOutput)
			err = ErrNotFound
		} else {
			err = &kubeCtlError{
				err:    errors.WithStack(err),
				cmd:    argsString,
				stderr: errOutput,
			}
		}
	}

	l.Debug(outBuf.String())
	l.Debug(errOutput)
	return outBuf.Bytes(), err
}
