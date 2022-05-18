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
	"context"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/percona-platform/dbaas-controller/utils/app"
)

func TestNewKubeCtl(t *testing.T) {
	t.Parallel()

	ctx := app.Context()

	kubeconfig, err := ioutil.ReadFile(os.Getenv("HOME") + "/.kube/config")
	require.NoError(t, err)

	t.Run("BasicNewKubeCtl", func(t *testing.T) {
		kubeCtl, err := NewKubeCtl(ctx, string(kubeconfig))
		require.NoError(t, err)
		// lookup for kubeconfig path
		var kubeconfigFlag string
		for _, option := range kubeCtl.cmd {
			if strings.HasPrefix(option, "--kubeconfig") {
				kubeconfigFlag = option

				break
			}
		}

		kubeconfigFilePath := strings.Split(kubeconfigFlag, "=")[1]
		kubeconfigActual, err := ioutil.ReadFile(kubeconfigFilePath) //nolint:gosec
		require.NoError(t, err)
		assert.Equal(t, kubeconfig, kubeconfigActual)

		err = kubeCtl.Cleanup()
		require.NoError(t, err)

		_, err = os.Stat(kubeCtl.kubeconfigPath)
		require.Error(t, err)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("BasicNewKubeCtl", func(t *testing.T) {
		kubeCtl, err := NewKubeCtl(ctx, "{")
		require.Error(t, err)
		// lookup for kubeconfig path
		require.NotNil(t, kubeCtl)
		assert.NoFileExists(t, kubeCtl.kubeconfigPath)
	})
}

const kubernetsVersions = `
{
	"clientVersion": {
	  "major": "1",
	  "minor": "23",
	  "gitVersion": "v1.23.6",
	  "gitCommit": "ad3338546da947756e8a88aa6822e9c11e7eac22",
	  "gitTreeState": "clean",
	  "buildDate": "2022-04-14T08:49:13Z",
	  "goVersion": "go1.17.9",
	  "compiler": "gc",
	  "platform": "linux/amd64"
	},
	"serverVersion": {
	  "major": "1",
	  "minor": "22",
	  "gitVersion": "v1.22.3",
	  "gitCommit": "c92036820499fedefec0f847e2054d824aea6cd1",
	  "gitTreeState": "clean",
	  "buildDate": "2021-10-27T18:35:25Z",
	  "goVersion": "go1.16.9",
	  "compiler": "gc",
	  "platform": "linux/amd64"
	}
}
`

func TestSelectCorrectKubectlVersions(t *testing.T) {
	t.Parallel()
	t.Run("basic", func(t *testing.T) {
		got, err := selectCorrectKubectlVersions([]byte(kubernetsVersions))
		require.NoError(t, err)
		expected := []string{
			dbaasToolPath + "/kubectl-1.23",
			dbaasToolPath + "/kubectl-1.22",
			dbaasToolPath + "/kubectl-1.21",
		}
		assert.Equal(t, got, expected)
	})

	t.Run("empty", func(t *testing.T) {
		got, err := selectCorrectKubectlVersions([]byte(""))
		assert.Errorf(t, err, "unexpected end of JSON input")
		assert.Nil(t, got)
	})
}

func TestGetKubectlCmd(t *testing.T) {
	t.Parallel()
	t.Run("basic", func(t *testing.T) {
		ctx := context.TODO()
		defaultKubectl, err := lookupCorrectKubectlCmd(nil, []string{defaultPmmServerKubectl, defaultDevEnvKubectl})
		require.NoError(t, err)
		got, err := getKubectlCmd(ctx, defaultKubectl, "")
		require.NoError(t, err)
		// `/usr/local/bin/minikube kubectl --` - for dev env
		// `/opt/dbaas-tools/bin/kubectl-1.23` - for pmm-server
		assert.Equal(t, got, defaultKubectl)
	})
}

func TestLookupCorrectKubectlCmd(t *testing.T) {
	t.Parallel()
	defaultKubectl, err := lookupCorrectKubectlCmd(nil, []string{defaultPmmServerKubectl, defaultDevEnvKubectl})
	require.NoError(t, err)
	t.Run("basic", func(t *testing.T) {
		args := []string{
			"kubectl-1.23",
			"kubectl-1.17",
			"kubectl-1.16",
			"kubectl-1.15",
		}
		got, err := lookupCorrectKubectlCmd(defaultKubectl, args)
		require.NoError(t, err)
		assert.Equal(t, got, defaultKubectl)
	})

	t.Run("empty_kubectl_list_of_correct_version", func(t *testing.T) {
		got, err := lookupCorrectKubectlCmd(defaultKubectl, nil)
		require.NoError(t, err)
		assert.Equal(t, got, defaultKubectl)
	})
}
