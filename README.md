# dbaas-controller

[![CI GitHub Action status](https://github.com/percona-platform/dbaas-controller/workflows/CI/badge.svg?branch=main)](https://github.com/percona-platform/dbaas-controller/actions?query=workflow%3ACI+branch%3Amain)
[![codecov.io Code Coverage](https://codecov.io/gh/percona-platform/dbaas-controller/branch/main/graph/badge.svg)](https://codecov.io/github/percona-platform/dbaas-controller?branch=main)
[![CLA assistant](https://cla-assistant.percona.com/readme/badge/percona-platform/dbaas-controller)](https://cla-assistant.percona.com/percona-platform/dbaas-controller)

dbaas-controller exposes a simplified API for managing Percona Kubernetes Operators.
#### Prerequisites

1. Installed minikube
2. Installed docker

#### Running minikube

To spin-up k8s cluster, run
```
    minikube start --cpus=4 --memory=7G --apiserver-names host.docker.internal --kubernetes-version=v1.23.0
    ENABLE_DBAAS=1 NETWORK=minikube make env-up # Run PMM with DBaaS feature enabled
```

[Read the documentation](https://docs.percona.com/percona-monitoring-and-management/setting-up/server/dbaas.html) how to run DBaaS on GKE or EKS

##### Troubleshooting

1. You can face with pod failing with `Init:CrashLoopBackOff` issue. Once you get logs by running `kubectl logs pxc-cluster-pxc-0 -c pxc-init` you get the error `install: cannot create regular file '/var/lib/mysql/pxc-entrypoint.sh': Permission denied`. You can fix it using [this solution](https://github.com/kubernetes/minikube/issues/12360#issuecomment-1123794143). Also, check [this issue](https://jira.percona.com/browse/K8SPXC-879)
2. Multinode PXC Cluster can't be created on ARM CPUs. You can have single node installation.
3. Operators are not supported. It means that the PMM version <-> operator version pair does not exist in the Version service. This issue can happen in two different scenarios. You can have a PMM version higher than the current release, or you installed a higher version of operators. You can check compatibility using https://check.percona.com/versions/v1/pmm-server/PMM-version


