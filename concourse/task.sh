#!/bin/bash

set -x
set -e

pushd environment
  keys_dir=$(mktemp -d)
  export BOSH_CA_CERT="${keys_dir}/bosh-ca.crt"
  export BOSH_CLIENT=$(bbl director-username)
  export BOSH_CLIENT_SECRET=$(bbl director-password)
  export BOSH_ENVIRONMENT=$(bbl director-address)
  export BOSH_GW_USER=vcap
  export BOSH_GW_HOST=$(bbl director-address | cut -d: -f2 | tr -d /)
  export BOSH_GW_PRIVATE_KEY="${keys_dir}/bosh.pem"

  bbl director-ca-cert > "${BOSH_CA_CERT}"
  bbl ssh-key > "${BOSH_GW_PRIVATE_KEY}"
  chmod 600 "${BOSH_GW_PRIVATE_KEY}"

  BBS_CLIENT_CERT_PATH="${PWD}/diego-certs/bbs-certs/client.crt"
  BBS_CLIENT_KEY_PATH="${PWD}/diego-certs/bbs-certs/client.key"
popd

config_path=$(mktemp -d)
export CONFIG=${config_path}/config.json

cat > "$CONFIG" <<EOF
{
  "api": "${CF_API_TARGET}",
  "admin_user": "admin",
  "admin_password": "${CF_ADMIN_PASSWORD}",
  "skip_ssl_validation": ${CF_SKIP_SSL_VALIDATION},
  "apps_domain": "${CF_APPS_DOMAIN}"
}
EOF

mkdir "${GOPATH}/src/code.cloudfoundry.org"
cp -a sync-integration-tests "${GOPATH}/src/code.cloudfoundry.org"

pushd "${GOPATH}/src/code.cloudfoundry.org/sync-integration-tests"
  ginkgo -nodes="${NODES}" --  \
    --bbs-client-cert="${BBS_CLIENT_CERT_PATH}" \
    --bbs-client-key="${BBS_CLIENT_KEY_PATH}" \
    --use-gateway
popd

exit 0
