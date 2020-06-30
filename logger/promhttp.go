package logger

import "github.com/prometheus/client_golang/prometheus/promhttp"

// PromHTTP is a compatibility wrapper between Logger interface and Prometheus HTTP logger interface.
type PromHTTP struct {
	L Logger
}

func (p *PromHTTP) Println(args ...interface{}) { p.L.Info(args...) } // nolint:golint

// check interfaces.
var (
	_ promhttp.Logger = (*PromHTTP)(nil)
)
