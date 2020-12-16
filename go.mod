module github.com/percona-platform/dbaas-controller

go 1.14

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
	github.com/gogo/protobuf v1.3.1 // indirect
	github.com/google/go-cmp v0.5.2 // indirect
	github.com/google/uuid v1.1.2
	github.com/grpc-ecosystem/go-grpc-middleware v1.2.2
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0
	github.com/kr/pretty v0.2.0 // indirect
	github.com/percona-platform/dbaas-api v0.0.0-20201216003837-68dead64fa93
	github.com/percona-platform/saas v0.0.0-20201208060513-b395fb361ebd
	github.com/percona/pmm v2.12.0+incompatible
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.8.0
	github.com/sirupsen/logrus v1.7.0
	github.com/stretchr/testify v1.6.1
	go.uber.org/zap v1.16.0
	golang.org/x/net v0.0.0-20201110031124-69a78807bb2b // indirect
	golang.org/x/sys v0.0.0-20201201145000-ef89a241ccb3
	golang.org/x/text v0.3.4
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	google.golang.org/grpc v1.34.0
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
)
