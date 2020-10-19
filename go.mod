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
	github.com/Azure/go-autorest/autorest v0.9.6 // indirect
	github.com/DATA-DOG/go-sqlmock v1.4.1 // indirect
	github.com/Percona-Lab/percona-version-service/api v0.0.0-20200714141734-e9fed619b55c // indirect
	github.com/Venafi/vcert v0.0.0-20200310111556-eba67a23943f // indirect
	github.com/aws/aws-sdk-go v1.35.9 // indirect
	github.com/blang/semver v3.5.1+incompatible // indirect
	github.com/brancz/gojsontoyaml v0.0.0-20190425155809-e8bd32d46b3d // indirect
	github.com/cpu/goacmedns v0.0.3 // indirect
	github.com/docker/spdystream v0.0.0-20181023171402-6480d4af844c // indirect
	github.com/evanphx/json-patch v4.9.0+incompatible // indirect
	github.com/fsnotify/fsnotify v1.4.9 // indirect
	github.com/ghodss/yaml v1.0.1-0.20190212211648-25d852aebe32 // indirect
	github.com/go-bindata/go-bindata v3.1.2+incompatible // indirect
	github.com/go-ini/ini v1.62.0
	github.com/go-logr/logr v0.2.1 // indirect
	github.com/go-openapi/runtime v0.19.16 // indirect
	github.com/gobuffalo/packr v1.30.1 // indirect
	github.com/golang/protobuf v1.4.3 // indirect
	github.com/golang/snappy v0.0.2 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/google/uuid v1.1.2
	github.com/grpc-ecosystem/go-grpc-middleware v1.2.2
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0
	github.com/hashicorp/go-version v1.2.1
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/helm/helm-2to3 v0.5.1 // indirect
	github.com/iancoleman/strcase v0.0.0-20190422225806-e506e3ef7365 // indirect
	github.com/imdario/mergo v0.3.11 // indirect
	github.com/jetstack/cert-manager v0.15.1
	github.com/jmoiron/sqlx v1.2.0 // indirect
	github.com/jsonnet-bundler/jsonnet-bundler v0.2.0 // indirect
	github.com/klauspost/compress v1.11.1 // indirect
	github.com/kylelemons/godebug v0.0.0-20170820004349-d65d576e9348 // indirect
	github.com/markbates/inflect v1.0.4 // indirect
	github.com/martinlindhe/base36 v1.0.0 // indirect
	github.com/mattn/go-isatty v0.0.12 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.2-0.20181231171920-c182affec369 // indirect
	github.com/miekg/dns v1.1.29 // indirect
	github.com/mikefarah/yq/v2 v2.4.1 // indirect
	github.com/mitchellh/hashstructure v1.0.0 // indirect
	github.com/moby/term v0.0.0-20200312100748-672ec06f55cd // indirect
	github.com/onsi/gomega v1.10.1 // indirect
	github.com/openshift/api v0.0.0-20200205133042-34f0ec8dab87 // indirect
	github.com/openshift/client-go v0.0.0-20190923180330-3b6373338c9b // indirect
	github.com/openshift/prom-label-proxy v0.1.1-0.20191016113035-b8153a7f39f1 // indirect
	github.com/operator-framework/api v0.1.1 // indirect
	github.com/otiai10/copy v1.0.2 // indirect
	github.com/percona-platform/dbaas-api v0.0.0-20201009095023-f103f733767a
	github.com/percona-platform/saas v0.0.0-20201008124851-3c2c6c2ec0ce
	github.com/percona/percona-backup-mongodb v1.3.2 // indirect
	github.com/percona/percona-server-mongodb-operator v1.4.0
	github.com/percona/percona-xtradb-cluster-operator v1.6.0
	github.com/percona/pmm v2.11.0+incompatible
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.8.0
	github.com/robfig/cron/v3 v3.0.1 // indirect
	github.com/rogpeppe/go-internal v1.5.0 // indirect
	github.com/rubenv/sql-migrate v0.0.0-20191025130928-9355dd04f4b3 // indirect
	github.com/sirupsen/logrus v1.7.0
	github.com/spf13/cobra v1.0.0 // indirect
	github.com/stoewer/go-strcase v1.2.0 // indirect
	github.com/stretchr/testify v1.6.1
	github.com/thanos-io/thanos v0.11.0 // indirect
	github.com/ziutek/mymysql v1.5.4 // indirect
	go.etcd.io/bbolt v1.3.5 // indirect
	go.etcd.io/etcd v0.5.0-alpha.5.0.20200819165624-17cef6e3e9d5 // indirect
	go.mongodb.org/mongo-driver v1.4.2 // indirect
	go.uber.org/multierr v1.6.0 // indirect
	go.uber.org/zap v1.16.0
	golang.org/x/crypto v0.0.0-20201016220609-9e8e0b390897 // indirect
	golang.org/x/net v0.0.0-20201016165138-7b1cca2348c0 // indirect
	golang.org/x/oauth2 v0.0.0-20200902213428-5d25da1a8d43 // indirect
	golang.org/x/sync v0.0.0-20201008141435-b3e1573b7520 // indirect
	golang.org/x/sys v0.0.0-20201018230417-eeed37f84f13
	golang.org/x/text v0.3.3
	golang.org/x/time v0.0.0-20200630173020-3af7569d3a1e // indirect
	gomodules.xyz/jsonpatch/v3 v3.0.1 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20201019141844-1ed22bb0c154 // indirect
	google.golang.org/grpc v1.33.0
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/gorp.v1 v1.7.2 // indirect
	helm.sh/helm/v3 v3.1.2 // indirect
	k8s.io/api v0.19.2
	k8s.io/apimachinery v0.19.2
	k8s.io/code-generator v0.19.0-alpha.1 // indirect
	k8s.io/gengo v0.0.0-20200428234225-8167cfdcfc14 // indirect
	k8s.io/klog/v2 v2.3.0 // indirect
	k8s.io/kube-state-metrics v1.7.2 // indirect
	k8s.io/utils v0.0.0-20201015054608-420da100c033 // indirect
	rsc.io/letsencrypt v0.0.3 // indirect
	sigs.k8s.io/apiserver-network-proxy/konnectivity-client v0.0.9 // indirect
	sigs.k8s.io/controller-runtime v0.5.6
	sigs.k8s.io/structured-merge-diff/v3 v3.0.0 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.0.1 // indirect
	software.sslmate.com/src/go-pkcs12 v0.0.0-20200619203921-c9ed90bd32dc // indirect
)

// Use the same versions as operators:
// * https://github.com/percona/percona-xtradb-cluster-operator/blob/master/go.mod
// * https://github.com/percona/percona-server-mongodb-operator/blob/master/go.mod
replace (
	k8s.io/api => k8s.io/api v0.17.4
	k8s.io/apimachinery => k8s.io/apimachinery v0.17.4
	k8s.io/client-go => k8s.io/client-go v0.17.4
)
