#!/bin/bash

PATH_TO_KUBECONFIG=${HOME}/.kube/config_eks
TOP_DIR=$(git rev-parse --show-toplevel)
PMM_USER="$(echo -n 'admin' | base64)";
PMM_PASS="$(echo -n 'admin_password' | base64)";
KUBECTL_CMD="kubectl --kubeconfig ${PATH_TO_KUBECONFIG}"

# See instructions in install_eks_operators.sh

# Delete the PXC operator
cat ${TOP_DIR}/deploy/pxc-operator.yaml | ${KUBECTL_CMD} delete -f -
cat ${TOP_DIR}/deploy/pxc-secrets.yaml | sed "s/pmmserver:.*=/pmmserver: ${PMM_PASS}/g" | ${KUBECTL_CMD} delete -f -

# Delete the PSMDB operator
cat ${TOP_DIR}/deploy/psmdb-operator.yaml | ${KUBECTL_CMD} delete -f -
cat ${TOP_DIR}/deploy/psmdb-secrets.yaml | sed "s/PMM_SERVER_USER:.*$/PMM_SERVER_USER: ${PMM_USER}/g;s/PMM_SERVER_PASSWORD:.*=$/PMM_SERVER_PASSWORD: ${PMM_PASS}/g;" | ${KUBECTL_CMD} delete -f -
