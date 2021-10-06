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
	  "minor": "16",
	  "gitVersion": "v1.16.8",
	  "gitCommit": "ec6eb119b81be488b030e849b9e64fda4caaf33c",
	  "gitTreeState": "clean",
	  "buildDate": "2020-03-12T21:00:06Z",
	  "goVersion": "go1.13.8",
	  "compiler": "gc",
	  "platform": "darwin/amd64"
	},
	"serverVersion": {
	  "major": "1",
	  "minor": "16",
	  "gitVersion": "v1.16.8",
	  "gitCommit": "ec6eb119b81be488b030e849b9e64fda4caaf33c",
	  "gitTreeState": "clean",
	  "buildDate": "2020-03-12T20:52:22Z",
	  "goVersion": "go1.13.8",
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
			dbaasToolPath + "/kubectl-1.17",
			dbaasToolPath + "/kubectl-1.16",
			dbaasToolPath + "/kubectl-1.15",
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
		// `/opt/dbaas-tools/bin/kubectl-1.16` - for pmm-server
		assert.Equal(t, got, defaultKubectl)
	})
}

func TestLookupCorrectKubectlCmd(t *testing.T) {
	t.Parallel()
	defaultKubectl, err := lookupCorrectKubectlCmd(nil, []string{defaultPmmServerKubectl, defaultDevEnvKubectl})
	require.NoError(t, err)
	t.Run("basic", func(t *testing.T) {
		args := []string{
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
