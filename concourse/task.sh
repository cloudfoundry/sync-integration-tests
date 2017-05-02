#!/bin/bash

set -e

config_path=$(mktemp -d)
export CONFIG=${config_path}/config.json

pushd environment
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
  "bbs_client_cert": "${PWD}/diego-certs/bbs-certs/client.crt",
  "bbs_client_key": "${PWD}/diego-certs/bbs-certs/client.key",
  "bosh_binary": "${BOSH_BINARY}",
  "bosh_api_instance": "${BOSH_API_INSTANCE}",
  "bosh_ca_cert": "${bosh_ca_cert}",
  "bosh_client": "$(bbl director-username)",
  "bosh_client_secret": "$(bbl director-password)",
  "bosh_environment": "$(bbl director-address)",
  "bosh_gw_user": "vcap",
  "bosh_gw_host": "$(bbl director-address | cut -d: -f2 | tr -d /)",
  "bosh_gw_private_key": "${bosh_gw_private_key}"
}
EOF
popd

mkdir "${GOPATH}/src/code.cloudfoundry.org"
cp -a sync-integration-tests "${GOPATH}/src/code.cloudfoundry.org"

pushd "${GOPATH}/src/code.cloudfoundry.org/sync-integration-tests"
  ginkgo -nodes="${NODES}"
popd

exit 0
