#!/bin/bash

PATH_TO_KUBECONFIG=${HOME}/.kube/config_eks
TOP_DIR=$(git rev-parse --show-toplevel)
PMM_USER="$(echo -n 'admin' | base64)";
PMM_PASS="$(echo -n 'admin_password' | base64)";
KUBECTL_CMD="kubectl --kubeconfig ${PATH_TO_KUBECONFIG}"

# This is to deploy the operators in an EKS cluster.
#
# Steps:
# - Install AWS CLI https://docs.aws.amazon.com/cli/latest/userguide/install-cliv2.html
# - Use `aws configure` and add `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` with your credentials.
# - Create a cluster with the following command  :
# - eksctl create cluster --write-kubeconfig —name=your-cluster-name —zones=us-west-2a,us-west-2b --kubeconfig <PATH_TO_KUBECONFIG>
# - Add your ACCESS and SECRET key from stage 3 to env section of your kube config file
# 
#  	env:
#   	- name: AWS_STS_REGIONAL_ENDPOINTS
#     	value: regional
#   	- name: AWS_DEFAULT_REGION
#     	value: us-west-2
#   	- name: AWS_ACCESS_KEY_ID
#     	value: XXXXXXXXXXXXXXXXXXXXXXXX
#   	- name: AWS_SECRET_ACCESS_KEY
#     	value: XXXXXXXXXXXXXXXXXXXXXXXX
# 
# - Replace aws to aws-iam-authenticator in users.user.exec.command of your kube config file and replace in args
#         - "eks"
#         - "get-token"
#         - "--cluster-name"
#         - "<cluster-name>"
# with
#        - "token"
#         - "-i"
# 

# Install the PXC operator
cat ${TOP_DIR}/deploy/pxc-operator.yaml | ${KUBECTL_CMD} apply -f -
cat ${TOP_DIR}/deploy/pxc-secrets.yaml | sed "s/pmmserver:.*=/pmmserver: ${PMM_PASS}/g" | ${KUBECTL_CMD} apply -f -

# Install the PSMDB operator
cat ${TOP_DIR}/deploy/psmdb-operator.yaml | ${KUBECTL_CMD} apply -f -
cat ${TOP_DIR}/deploy/psmdb-secrets.yaml | sed "s/PMM_SERVER_USER:.*$/PMM_SERVER_USER: ${PMM_USER}/g;s/PMM_SERVER_PASSWORD:.*=$/PMM_SERVER_PASSWORD: ${PMM_PASS}/g;" | ${KUBECTL_CMD} apply -f -

