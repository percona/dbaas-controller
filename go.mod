module github.com/percona-platform/dbaas-controller

go 1.16

// Use for local development, but do not commit:
// replace github.com/percona/pmm => ../pmm
// replace github.com/percona-platform/dbaas-api => ../dbaas-api
// replace github.com/percona-platform/saas => ../saas

// Update with:
// go get -v github.com/percona/pmm@latest (for the latest tag) or @PMM-2.0 (only if really needed)
// go get -v github.com/percona-platform/dbaas-api@main
// go get -v github.com/percona-platform/saas@main

require (
	github.com/AlekSi/pointer v1.1.0
	github.com/google/uuid v1.2.0
	github.com/grpc-ecosystem/go-grpc-middleware v1.2.2
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0
	github.com/hashicorp/go-version v1.3.0
	github.com/percona-platform/dbaas-api v0.0.0-20211201151251-014259873599
	github.com/percona-platform/saas v0.0.0-20201127072600-f1ffa53f7871
	github.com/percona/pmm v2.15.1-0.20210318204615-bbf8e9314afd+incompatible
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.10.0
	github.com/sirupsen/logrus v1.8.1
	github.com/stretchr/testify v1.7.0
	go.uber.org/zap v1.16.0
	golang.org/x/sys v0.0.0-20210831042530-f4d43177bf5e
	golang.org/x/text v0.3.7
	google.golang.org/grpc v1.38.0
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	k8s.io/client-go v0.23.0
)
