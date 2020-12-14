#!/bin/bash

TOP_DIR=$(git rev-parse --show-toplevel)
PMM_USER="$(echo -n 'admin' | base64)";
PMM_PASS="$(echo -n 'admin_password' | base64)";
KUBECTL_CMD="kubectl --kubeconfig ${HOME}/.kube/config_eks"

# Install the PXC operator
cat ${TOP_DIR}/deploy/pxc_operator.yaml | ${KUBECTL_CMD} apply -f -
cat ${TOP_DIR}/deploy/secrets.yaml | sed "s/pmmserver:.*=/pmmserver: ${PMM_PASS}/g" | ${KUBECTL_CMD} apply -f -

# Install the PSMDB operator
cat ${TOP_DIR}/deploy/psmdb_operator.yaml | ${KUBECTL_CMD} apply -f -
cat ${TOP_DIR}/deploy/secrets.yaml | sed "s/PMM_SERVER_USER:.*$/PMM_SERVER_USER: ${PMM_USER}/g;s/PMM_SERVER_PASSWORD:.*=$/PMM_SERVER_PASSWORD: ${PMM_PASS}/g;" | ${KUBECTL_CMD} apply -f -

