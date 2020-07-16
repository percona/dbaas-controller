KUBERNETES_VERSION ?= 1.16.8

default: help

help:                             ## Display this help message
	@echo "Please use \`make <target>\` where <target> is one of:"
	@grep '^[a-zA-Z]' $(MAKEFILE_LIST) | \
		awk -F ':.*?## ' 'NF==2 {printf "  %-26s%s\n", $$1, $$2}'

# `cut` is used to remove first `v` from `git describe` output
PMM_RELEASE_PATH ?= bin
PMM_RELEASE_COMPONENT_VERSION ?= $(shell git describe --always --dirty | cut -b2-)
PMM_RELEASE_VERSION ?= $(shell git describe --always --dirty | cut -b2-)
PMM_RELEASE_TIMESTAMP ?= $(shell date '+%s')
PMM_RELEASE_FULLCOMMIT ?= $(shell git rev-parse HEAD)
PMM_RELEASE_BRANCH ?= $(shell git describe --always --contains --all)

PMM_LD_FLAGS = -ldflags " \
			-X 'github.com/percona/pmm/version.ProjectName=dbaas-controller' \
			-X 'github.com/percona/pmm/version.Version=$(PMM_RELEASE_COMPONENT_VERSION)' \
			-X 'github.com/percona/pmm/version.PMMVersion=$(PMM_RELEASE_VERSION)' \
			-X 'github.com/percona/pmm/version.Timestamp=$(PMM_RELEASE_TIMESTAMP)' \
			-X 'github.com/percona/pmm/version.FullCommit=$(PMM_RELEASE_FULLCOMMIT)' \
			-X 'github.com/percona/pmm/version.Branch=$(PMM_RELEASE_BRANCH)' \
			"

release:                          ## Build dbaas-controller release binaries.
	env CGO_ENABLED=0 go build -mod=readonly -v $(PMM_LD_FLAGS) -o $(PMM_RELEASE_PATH)/dbaas-controller ./cmd/dbaas-controller

init:                             ## Install development tools
	go build -o bin/check-license ./.github/check-license.go

	go build -modfile=tools/go.mod -o bin/go-consistent github.com/quasilyte/go-consistent
	go build -modfile=tools/go.mod -o bin/goimports golang.org/x/tools/cmd/goimports
	go build -modfile=tools/go.mod -o bin/golangci-lint github.com/golangci/golangci-lint/cmd/golangci-lint
	go build -modfile=tools/go.mod -o bin/gotext golang.org/x/text/cmd/gotext
	go build -modfile=tools/go.mod -o bin/reviewdog github.com/reviewdog/reviewdog/cmd/reviewdog

gen:                              ## Generate code
	go generate ./catalog
	mv catalog/locales/en/out.gotext.json catalog/locales/en/messages.gotext.json
	# add blank line at EOF
	echo >> catalog/locales/en/messages.gotext.json
	make format

format:                           ## Format source code
	gofmt -w -s .
	bin/goimports -local github.com/percona-platform/dbaas-controller -l -w .

check:                            ## Run checks/linters for the whole project
	bin/check-license
	bin/go-consistent -pedantic ./...
	bin/golangci-lint run

install:                          ## Install binaries
	go build -race -o bin/dbaas-controller ./cmd/dbaas-controller

test:                             ## Run tests
	go test -race -timeout=15m ./...

test-cover:                       ## Run tests and collect per-package coverage information
	go test -race -timeout=15m -count=1 -coverprofile=cover.out -covermode=atomic ./...

test-crosscover:                  ## Run tests and collect cross-package coverage information
	go test -race -timeout=15m -count=1 -coverprofile=crosscover.out -covermode=atomic -p=1 -coverpkg=./... ./...

run: install                      ## Run dbaas-controller
	bin/dbaas-controller

env-up:                           ## Start development environment
	make env-up-start
	make env-up-wait

env-up-start:
	minikube config set kubernetes-version $(KUBERNETES_VERSION)
	minikube config view
	minikube start
	minikube status
	minikube profile list
	minikube addons list
	minikube kubectl -- version
	curl -sSf -m 30 https://raw.githubusercontent.com/percona/percona-xtradb-cluster-operator/release-1.4.0/deploy/bundle.yaml  | minikube kubectl -- apply -f -
	curl -sSf -m 30 https://raw.githubusercontent.com/percona/percona-xtradb-cluster-operator/release-1.4.0/deploy/secrets.yaml | minikube kubectl -- apply -f -
	minikube kubectl -- get nodes
	minikube kubectl -- get pods

env-up-wait:
	minikube kubectl -- wait --for=condition=Available deployment percona-xtradb-cluster-operator

env-down:
	#
	# Please use `minikube stop` to stop Kubernetes cluster, or `minikube delete` to fully delete it.
	# Not picking one for you.
	#

collect-debugdata:                ## Collect debugdata
	rm -fr debugdata
	mkdir debugdata
	minikube logs --length=100 > ./debugdata/minikube.txt
	minikube kubectl -- describe pods > ./debugdata/pods.txt
	minikube kubectl -- describe pv,pvc > ./debugdata/pv-pvc.txt
	minikube kubectl -- get events --sort-by=lastTimestamp > ./debugdata/events.txt
	minikube kubectl -- logs --all-containers --timestamps --selector='name=percona-xtradb-cluster-operator' > ./debugdata/pxc-operators.txt
	minikube kubectl -- logs --all-containers --timestamps --selector='app.kubernetes.io/name=percona-xtradb-cluster' > ./debugdata/pxc-clusters.txt
