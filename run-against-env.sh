#!/bin/bash

set -exu

# ENV
main() {
  : "${BOSH_BINARY:="bosh"}"
  : "${BOSH_DEPLOYMENT_NAME:="cf"}"
  : "${BOSH_API_INSTANCE:="api/0"}"
  : "${CF_SKIP_SSL_VALIDATION:="true"}"
  : "${DEPLOYMENT_VARS_FILENAME:="deployment-vars.yml"}"
  : "${RUN_ROUTING_TESTS:="false"}"
  : "${VARS_STORE_PATH:=""}"
  :  BBL_STATE_DIR
  :  CF_APPS_DOMAIN

  # INPUTS
  config_dir=$(mktemp -d /tmp/sits-config.XXXXXX)
  export CONFIG=${config_dir}/config.json
  echo "$config_dir"

  bosh_certs_dir=$(mktemp -d /tmp/sits-bosh-certs.XXXXXX)
  mkdir -p "${bosh_certs_dir}/diego-certs/bbs-certs"
  bbs_cert_path="${bosh_certs_dir}/diego-certs/bbs-certs/client.crt"
  bbs_key_path="${bosh_certs_dir}/diego-certs/bbs-certs/client.key"

  set +x
    if [[ ! -z ${VARS_STORE_PATH} ]]; then
      pushd "${VARS_STORE_PATH}" > /dev/null
        CF_ADMIN_PASSWORD="$(bosh int --path /cf_admin_password ${DEPLOYMENT_VARS_FILENAME})"
        bosh int --path /diego_bbs_client/certificate "${DEPLOYMENT_VARS_FILENAME}" > "${bbs_cert_path}"
        bosh int --path /diego_bbs_client/private_key "${DEPLOYMENT_VARS_FILENAME}" > "${bbs_key_path}"
      popd > /dev/null
    else
      CF_ADMIN_PASSWORD="$(fetch_credhub_cred cf_admin_password "")"
      echo "$(fetch_credhub_cred diego_bbs_client ".certificate")" > "${bbs_cert_path}"
      echo "$(fetch_credhub_cred diego_bbs_client ".private_key")" > "${bbs_key_path}"
    fi
  set -x

  pushd "${BBL_STATE_DIR}" > /dev/null
    set +x
      keys_dir=$(mktemp -d /tmp/sits-keys-dir.XXXXXX)
      bosh_ca_cert="${keys_dir}/bosh-ca.crt"
      bbl director-ca-cert > "${bosh_ca_cert}"
      bosh_gw_private_key="${keys_dir}/bosh.pem"
      bbl ssh-key > "${bosh_gw_private_key}"
      chmod 600 "${bosh_gw_private_key}"

      cat > "$CONFIG" <<EOF
{
  "cf_api": "api.${CF_SYSTEM_DOMAIN}",
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
  "bosh_gw_user": "jumpbox",
  "bosh_gw_host": "$(bbl director-address | cut -d: -f2 | tr -d /)",
  "bosh_gw_private_key": "${bosh_gw_private_key}"
}
EOF
    set -x
  popd > /dev/null

  cd $(dirname "${BASH_SOURCE[0]}")/..

  go install code.cloudfoundry.org/sync-integration-tests/vendor/github.com/onsi/ginkgo/ginkgo
  ginkgo -r -nodes=3 -randomizeAllSpecs

  rm -r "${config_dir}"
  rm -r "${bosh_certs_dir}"
  rm -r "${keys_dir}"

  exit 0
}

fetch_credhub_cred() {
  var=$1
  field=$2

  credhub find -j -n "${var}" | jq -r .credentials[].name | xargs credhub get -j -n | jq -r ".value${field}"
}

main
