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
	"sync"
	"time"

	"github.com/percona/pmm/utils/pdeathsig"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"

	gocache "github.com/patrickmn/go-cache"

	"github.com/percona-platform/dbaas-controller/utils/logger"
)

// Option represents kubectl option, it's meant to be set of bitmasks.
type Option uint64

const (
	dbaasToolPath           = "/opt/dbaas-tools/bin"
	defaultPmmServerKubectl = dbaasToolPath + "/kubectl-1.16"
	defaultDevEnvKubectl    = "minikube kubectl --"
	expirationTime          = time.Second * 10
	// UseCacheOption is an option that turns on cache of kubectl commands.
	UseCacheOption Option = 1
)

// KubeCtl wraps kubectl CLI with version selection and kubeconfig handling.
type KubeCtl struct {
	l              logger.Logger
	cmd            []string
	kubeconfigPath string
	daemonMutex    *sync.Mutex
	daemonCmdMutex *sync.Mutex
	daemonCmd      *exec.Cmd
	cache          *gocache.Cache
}

// NewKubeCtl creates a new KubeCtl object with a given logger.
func NewKubeCtl(ctx context.Context, kubeconfig string, options ...Option) (*KubeCtl, error) {
	l := logger.Get(ctx)
	l = l.WithField("component", "kubectl")

	// Firstly lookup default kubectl to get Kubernetes Server version.
	defKubectls := []string{defaultPmmServerKubectl, defaultDevEnvKubectl}
	defaultKubectl, err := lookupCorrectKubectlCmd(nil, defKubectls)
	if err != nil {
		return nil, err
	}

	var cache *gocache.Cache
	if len(options) == 1 && options[0]&UseCacheOption != 0 {
		l.Info("kubectl cache is turned on")
		// Setup cache
		cache = gocache.New(expirationTime, expirationTime*2)
	}

	// Cannot identify k8s server version on non local env without kubeconfig (w/o address of k8s server).
	if kubeconfig == "" {
		return &KubeCtl{
			l:              l,
			cmd:            defaultKubectl,
			daemonMutex:    new(sync.Mutex),
			daemonCmdMutex: new(sync.Mutex),
			cache:          cache,
		}, nil
	}

	// Handle kubeconfig.
	kubeconfigPath, err := saveKubeconfig(kubeconfig)
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
		l:              l,
		cmd:            cmd,
		kubeconfigPath: kubeconfigPath,
		daemonMutex:    new(sync.Mutex),
		daemonCmdMutex: new(sync.Mutex),
		cache:          cache,
	}, nil
}

func saveKubeconfig(kubeconfig string) (string, error) {
	tmpFile, err := ioutil.TempFile("", "dbaas-controller-kubeconfig-*")
	if err != nil {
		return "", err
	}

	_, err = tmpFile.Write([]byte(kubeconfig))
	if err != nil {
		return "", err
	}

	if err := tmpFile.Close(); err != nil {
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

	stdout, err := k.Run(ctx, args, nil)
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

// Delete executes `kubectl delete` with given resource.
func (k *KubeCtl) Delete(ctx context.Context, res interface{}) error {
	_, err := run(ctx, k.cmd, []string{"delete", "-f", "-"}, res)
	return err
}

// Run wraps func run.
func (k *KubeCtl) Run(ctx context.Context, args []string, stdin interface{}) ([]byte, error) {
	var argsString string
	if k.cache != nil {
		if len(args) > 0 && strings.ToLower(args[0]) == "get" || strings.ToLower(args[0]) == "describe" {
			argsString = strings.Join(args, " ")
			if bytes, found := k.cache.Get(argsString); found {
				k.l.Debugf("Returning cached response for '%s'", argsString)
				return bytes.([]byte), nil
			}
		}
	}
	out, err := run(ctx, k.cmd, args, stdin)
	if err != nil {
		return nil, err
	}
	if argsString != "" {
		k.cache.Set(argsString, out, gocache.DefaultExpiration)
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
	envs := os.Environ()
	for _, env := range envs {
		if strings.HasPrefix(env, "PATH=") {
			env = fmt.Sprintf("PATH=%s:%s", dbaasToolPath, os.Getenv("PATH"))
		}
		cmd.Env = append(cmd.Env, env)
	}
	err := cmd.Run()
	if err != nil {
		if strings.Contains(errBuf.String(), "NotFound") {
			err = ErrNotFound
		} else {
			err = &kubeCtlError{
				err:    errors.WithStack(err),
				cmd:    argsString,
				stderr: errBuf.String(),
			}
		}
	}

	l.Debug(outBuf.String())
	l.Debug(errBuf.String())
	return outBuf.Bytes(), err
}

// RunDaemon runs kubectl command with args and assigns the process to daemonCmd
// so we can later kill it. This call blocks. StopDaemon has to be run when daemon
// is no longer needed. Concurrent calls to this method blocks until the current
// daemon is stopped.
func (k *KubeCtl) RunDaemon(ctx context.Context, args ...string) error {
	// don't start another daemon until current stops.
	k.daemonMutex.Lock()
	defer k.daemonMutex.Unlock()
	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	args = append(k.cmd, args...)
	cmd := exec.CommandContext(ctx, args[0], args[1:]...) //nolint:gosec
	pdeathsig.Set(cmd, unix.SIGKILL)
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	envs := os.Environ()
	for _, env := range envs {
		if strings.HasPrefix(env, "PATH=") {
			env = fmt.Sprintf("PATH=%s:%s", dbaasToolPath, os.Getenv("PATH"))
		}
		cmd.Env = append(cmd.Env, env)
	}
	k.daemonCmdMutex.Lock()
	k.daemonCmd = cmd
	k.daemonCmdMutex.Unlock()
	err := cmd.Run()
	if err != nil {
		if strings.Contains(errBuf.String(), "NotFound") {
			return ErrNotFound
		}
		return &kubeCtlError{
			err:    errors.WithStack(err),
			cmd:    strings.Join(args, " "),
			stderr: errBuf.String(),
		}
	}
	k.daemonCmdMutex.Lock()
	k.daemonCmd = nil
	k.daemonCmdMutex.Unlock()
	return nil
}

// StopDaemon stops current daemon if there is any running.
func (k *KubeCtl) StopDaemon() error {
	k.daemonCmdMutex.Lock()
	defer k.daemonCmdMutex.Unlock()
	if k.daemonCmd == nil {
		return nil
	}
	return k.daemonCmd.Process.Kill()
}
