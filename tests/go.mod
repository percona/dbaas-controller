module github.com/percona-platform/dbaas-controller/tests

go 1.16

// Use for local development, but do not commit:
// replace github.com/percona-platform/dbaas-api => ../../dbaas-api

// Update with:
// go get -v github.com/percona-platform/dbaas-api@main

require (
	github.com/percona-platform/dbaas-api v0.0.0-20210630090346-f95d2ec8b7c1
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.8.0
	github.com/stretchr/testify v1.7.0
	golang.org/x/net v0.0.0-20200520004742-59133d7f0dd7 // indirect
	golang.org/x/sys v0.0.0-20201029080932-201ba4db2418
	google.golang.org/grpc v1.38.0
)
