package kube

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKubeClient(t *testing.T) {
	kubeconfig, err := ioutil.ReadFile(os.Getenv("HOME") + "/.kube/config")
	require.NoError(t, err)
	_, err = NewFromKubeConfigObject(string(kubeconfig))
	assert.NoError(t, err)
}
