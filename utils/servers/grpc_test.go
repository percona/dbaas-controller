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

package servers

import (
	"context"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/percona-platform/dbaas-controller/service/k8sclient"
)

type kubeAuth struct {
	Kubeconfig string
}

type nonAuthReq struct{}

type testReq struct {
	KubeAuth kubeAuth
}

type trapReq struct {
	KubeAuth string
}

type testResp struct{}

type pointerReq struct {
	KubeAuth *kubeAuth
}

var errNotInjected error = errors.New("k8sclient was not injected")

func handler(ctx context.Context, r interface{}) (interface{}, error) {
	if _, ok := ctx.Value(K8sClientKey).(*k8sclient.K8sClient); !ok {
		return nil, errNotInjected
	}
	return testResp{}, nil
}

func TestInjectK8sClient(t *testing.T) {
	t.Parallel()
	t.Run("Request with KubeAuth.Kubeconfig", func(t *testing.T) {
		t.Parallel()

		r := testReq{
			KubeAuth: kubeAuth{
				Kubeconfig: "",
			},
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		resp, err := injectK8sClient(ctx, r, nil, handler)
		require.NoError(t, err)
		_, ok := resp.(testResp)
		assert.True(t, ok)
	})

	t.Run("Request without KubeAuth.Kubeconfig", func(t *testing.T) {
		t.Parallel()
		r := nonAuthReq{}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		resp, err := injectK8sClient(ctx, r, nil, handler)
		assert.ErrorIs(t, err, errNotInjected)
		_, ok := resp.(testResp)
		assert.False(t, ok)
	})

	t.Run("Request with KubeAuth but without Kubeconfig", func(t *testing.T) {
		t.Parallel()
		r := trapReq{}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		resp, err := injectK8sClient(ctx, r, nil, handler)
		assert.ErrorIs(t, err, errNotInjected)
		_, ok := resp.(testResp)
		assert.False(t, ok)
	})
	t.Run("Request with pointers", func(t *testing.T) {
		t.Parallel()

		r := &pointerReq{
			KubeAuth: &kubeAuth{
				Kubeconfig: "",
			},
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		resp, err := injectK8sClient(ctx, r, nil, handler)
		require.NoError(t, err)
		_, ok := resp.(testResp)
		assert.True(t, ok)
	})

}
