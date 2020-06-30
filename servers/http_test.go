package servers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHTTPServer(t *testing.T) {
	t.Run("InvalidAddr", func(t *testing.T) {
		require.Panics(t, func() {
			RunHTTPServer(context.Background(), &RunHTTPServerOpts{
				Addr: "invalid.port:99999",
			})
		})
	})
}
