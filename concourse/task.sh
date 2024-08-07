#!/bin/bash

set -eu

# ENV
: "${BBL_STATE_DIR:=""}"
: "${VARS_STORE_PATH:=""}"
: "${USE_CF_DEPLOYMENT_VARS:="false"}"
: "${USE_CREDHUB:="false"}"
: "${RUN_REVISIONS_TESTS:="false"}"
: "${FLAKE_ATTEMPTS:=1}"

get_from_credhub() {
  set +x
  local credential_name=$1
  local credential_section=$2
  credhub find -j -n ${credential_name} | jq -r .credentials[].name | xargs credhub get -j -n | jq -r .${credential_section}
}

# INPUTS
script_dir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
workspace_dir="$( cd "${script_dir}/../../" && pwd )"
vars_store_dir="${workspace_dir}/vars-store" # optional

config_path=$(mktemp -d)
export CONFIG=${config_path}/config.json

pushd "environment/${BBL_STATE_DIR}" > /dev/null
  eval "$(bbl print-env)"

  mkdir -p "${PWD}/diego-certs/bbs-certs"
  bbs_cert_path="${PWD}/diego-certs/bbs-certs/client.crt"
  bbs_key_path="${PWD}/diego-certs/bbs-certs/client.key"

  if [ "${USE_CF_DEPLOYMENT_VARS}" = "true" ]; then
    vars_store_file="${vars_store_dir}/${VARS_STORE_PATH}"
    CF_ADMIN_PASSWORD="$(bosh int --path /cf_admin_password ${vars_store_file})"
    bosh int --path /diego_bbs_client/certificate "${vars_store_file}" > "${bbs_cert_path}"
    bosh int --path /diego_bbs_client/private_key "${vars_store_file}" > "${bbs_key_path}"
  elif [ "${USE_CREDHUB}" = "true" ]; then
    CF_ADMIN_PASSWORD="$(get_from_credhub cf_admin_password value)"
    get_from_credhub diego_bbs_client value.certificate > "${bbs_cert_path}"
    get_from_credhub diego_bbs_client value.private_key > "${bbs_key_path}"
  fi

  keys_dir=$(mktemp -d)
  bosh_ca_cert="${keys_dir}/bosh-ca.crt"
  bbl director-ca-cert > "${bosh_ca_cert}"
  bosh_gw_private_key="${keys_dir}/bosh.pem"
  bbl ssh-key > "${bosh_gw_private_key}"
  chmod 600 "${bosh_gw_private_key}"

  cat > "$CONFIG" <<EOF
{
  "cf_api": "${CF_API_TARGET}",
  "cf_admin_user": "admin",
  "cf_admin_password": "${CF_ADMIN_PASSWORD}",
  "cf_skip_ssl_validation": ${CF_SKIP_SSL_VALIDATION},
  "cf_apps_domain": "${CF_APPS_DOMAIN}",
  "bbs_client_cert": "${bbs_cert_path}",
  "bbs_client_key": "${bbs_key_path}",
  "bosh_binary": "${BOSH_BINARY}",
  "bosh_api_instance": "${BOSH_API_INSTANCE}",
  "bosh_deployment_name": "${BOSH_DEPLOYMENT_NAME}",
  "bosh_ca_cert": "${bosh_ca_cert}",
  "bosh_client": "$(bbl director-username)",
  "bosh_client_secret": "$(bbl director-password)",
  "bosh_environment": "$(bbl director-address)",
  "bosh_gw_user": "${BOSH_GATEWAY_USER}",
  "bosh_gw_host": "$(bbl director-address | cut -d: -f2 | tr -d /)",
  "bosh_gw_private_key": "${bosh_gw_private_key}",
  "run_revisions_tests": ${RUN_REVISIONS_TESTS}
}
EOF
popd > /dev/null

mkdir -p "${GOPATH}/src/code.cloudfoundry.org"
cp -a sync-integration-tests "${GOPATH}/src/code.cloudfoundry.org"

pushd "${GOPATH}/src/code.cloudfoundry.org/sync-integration-tests" > /dev/null
  go install github.com/onsi/ginkgo/v2/ginkgo

  ginkgo -nodes="${NODES}" --flake-attempts="${FLAKE_ATTEMPTS}"
popd > /dev/null

exit 0
