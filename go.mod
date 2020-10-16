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
	github.com/docker/spdystream v0.0.0-20181023171402-6480d4af844c // indirect
	github.com/go-ini/ini v1.62.0
	github.com/google/go-cmp v0.5.2 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.1.2
	github.com/grpc-ecosystem/go-grpc-middleware v1.2.2
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0
	github.com/hashicorp/go-version v1.2.1
	github.com/imdario/mergo v0.3.11 // indirect
	github.com/jetstack/cert-manager v1.0.0-beta.0
	github.com/matttproud/golang_protobuf_extensions v1.0.2-0.20181231171920-c182affec369 // indirect
	github.com/percona-platform/dbaas-api v0.0.0-20201009095023-f103f733767a
	github.com/percona-platform/saas v0.0.0-20201008124851-3c2c6c2ec0ce
	github.com/percona/percona-backup-mongodb v1.2.0 // indirect
	github.com/percona/percona-server-mongodb-operator v1.4.0
	github.com/percona/percona-xtradb-cluster-operator v1.6.0
	github.com/percona/pmm v2.10.1+incompatible
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.7.1
	github.com/sirupsen/logrus v1.6.0
	github.com/stretchr/testify v1.6.1
	go.uber.org/multierr v1.6.0 // indirect
	go.uber.org/zap v1.16.0
	golang.org/x/crypto v0.0.0-20201012173705-84dcc777aaee // indirect
	golang.org/x/net v0.0.0-20201010224723-4f7140c49acb // indirect
	golang.org/x/oauth2 v0.0.0-20200902213428-5d25da1a8d43 // indirect
	golang.org/x/sys v0.0.0-20201014080544-cc95f250f6bc
	golang.org/x/text v0.3.3
	golang.org/x/time v0.0.0-20200630173020-3af7569d3a1e // indirect
	golang.org/x/tools v0.0.0-20201015182029-a5d9e455e9c4 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/grpc v1.32.0
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	k8s.io/api v0.19.2
	k8s.io/apimachinery v0.19.2
	k8s.io/utils v0.0.0-20201005171033-6301aaf42dc7 // indirect
	sigs.k8s.io/controller-runtime v0.6.3
)

// Use the same versions as operators:
// * https://github.com/percona/percona-xtradb-cluster-operator/blob/master/go.mod
// * https://github.com/percona/percona-server-mongodb-operator/blob/master/go.mod
replace (
	k8s.io/api => k8s.io/api v0.17.4
	k8s.io/apimachinery => k8s.io/apimachinery v0.17.4
	k8s.io/client-go => k8s.io/client-go v0.17.4
)
