module github.com/percona-platform/dbaas-controller/tests

go 1.14

// Use for local development, but do not commit:
// replace github.com/percona-platform/dbaas-api => ../dbaas-api

// Update with:
// go get -v github.com/percona-platform/dbaas-api@main

require (
	github.com/percona-platform/dbaas-api v0.0.0-20201217181941-cc1aea155a80
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.7.0
	github.com/stretchr/testify v1.6.1
	golang.org/x/net v0.0.0-20200520004742-59133d7f0dd7 // indirect
	golang.org/x/sys v0.0.0-20201029080932-201ba4db2418
	google.golang.org/grpc v1.34.0
)
