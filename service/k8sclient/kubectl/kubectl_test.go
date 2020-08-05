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
	"crypto/sha256"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/percona-platform/dbaas-controller/utils/app"
	"github.com/percona-platform/dbaas-controller/utils/logger"
)

func SetUp(t *testing.T) string {
	cmd := []string{"dbaas-kubectl-1.16"}
	kubectlPath, err := exec.LookPath(cmd[0])
	cmd = []string{kubectlPath}
	if e, ok := err.(*exec.Error); err != nil && ok && e.Err == exec.ErrNotFound {
		cmd = []string{"minikube", "kubectl", "--"}
	}
	cmd = append(cmd, "config", "view", "-o", "json")
	validKubeconfig, err := exec.Command(cmd[0], cmd[1:]...).Output() //nolint:gosec
	require.NoError(t, err)
	return string(validKubeconfig)
}

func TestNewKubeCtl(t *testing.T) {
	validKubeconfig := SetUp(t)
	logger.SetupGlobal()

	ctx := app.Context()
	l := logger.Get(ctx).WithField("component", "kubectl")

	t.Run("BasicNewKubeCtl", func(t *testing.T) {
		sha256KubeconfigExpected := sha256.Sum256([]byte(validKubeconfig))
		kubeCtl, err := NewKubeCtl(l, validKubeconfig)
		require.NoError(t, err)
		// lookup for kubeconfig path
		var kubeconfigFlag string
		for _, option := range kubeCtl.cmd {
			if strings.HasPrefix(option, "--kubeconfig") {
				kubeconfigFlag = option
				break
			}
		}

		assert.True(t, strings.HasSuffix(kubeconfigFlag, kubeconfigFileName))

		kubeconfigFilePath := strings.Split(kubeconfigFlag, "=")[1]
		dat, err := ioutil.ReadFile(kubeconfigFilePath) //nolint:gosec
		require.NoError(t, err)
		sha256KubeconfigActual := sha256.Sum256(dat)
		assert.Equal(t, sha256KubeconfigExpected, sha256KubeconfigActual)

		tmpDir := strings.TrimSuffix(kubeconfigFilePath, "/"+kubeconfigFileName)
		assert.Equal(t, kubeCtl.tmpDir, tmpDir)

		err = kubeCtl.Cleanup()
		require.NoError(t, err)

		_, err = os.Stat(tmpDir)
		require.Error(t, err)
		assert.True(t, os.IsNotExist(err))
	})
}
