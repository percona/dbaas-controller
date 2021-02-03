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

	"github.com/percona-platform/dbaas-controller/service/k8sclient"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type kubeAuth struct {
	Kubeconfig string
}

type nonAuthReq struct {
}

type testReq struct {
	KubeAuth kubeAuth
}

type testResp struct {
}

var errNotInjected = errors.New("k8sclient was not injected")

func handler(ctx context.Context, r interface{}) (interface{}, error) {
	if _, ok := ctx.Value("k8sclient").(*k8sclient.K8sClient); !ok {
		return nil, errNotInjected
	}
	return testResp{}, nil
}

func TestInjectK8sClient(t *testing.T) {
	t.Run("Request with KubeAuth.Kubeconfig", func(t *testing.T) {
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
		r := nonAuthReq{}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		resp, err := injectK8sClient(ctx, r, nil, handler)
		assert.ErrorIs(t, err, errNotInjected)
		_, ok := resp.(testResp)
		assert.False(t, ok)
	})

}
