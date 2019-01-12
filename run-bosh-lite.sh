#!/bin/bash

set -eu

fetch_credhub_cred() {
  var=$1
  field=$2

  credhub find -j -n "${var}" | jq -r .credentials[].name | xargs credhub get -j -n | jq -r ".value${field}"
}

if [ $(uname) == 'Darwin' ]; then
  config_path=$(mktemp -d -t 'sits')
else
  config_path=$(mktemp -d)
fi

bbs_client_cert_path="${config_path}/bbs.crt"
bbs_client_key_path="${config_path}/bbs.key"
echo "$(fetch_credhub_cred diego_bbs_client ".certificate")" > "${bbs_client_cert_path}"
echo "$(fetch_credhub_cred diego_bbs_client ".private_key")" > "${bbs_client_key_path}"

director_ca_cert="${config_path}/director.crt"
echo "${BOSH_CA_CERT}" > "${director_ca_cert}"

director_gw_key="${config_path}/director_gw.key"
echo "${BOSH_GW_PRIVATE_KEY_CONTENTS}" > "${director_gw_key}"

chmod 600 "${director_gw_key}"

export CONFIG=${config_path}/config.json
cat > "$CONFIG" <<EOF
{
  "cf_api": "api.${BOSH_LITE_DOMAIN}",
  "cf_admin_user": "admin",
  "cf_admin_password": "$(fetch_credhub_cred cf_admin_password "")",
  "cf_skip_ssl_validation": true,
  "cf_apps_domain": "${BOSH_LITE_DOMAIN}",
  "bbs_client_cert": "${bbs_client_cert_path}",
  "bbs_client_key": "${bbs_client_key_path}",
  "bosh_ca_cert": "${director_ca_cert}",
  "bosh_client": "admin",
  "bosh_client_secret": "${BOSH_CLIENT_SECRET}",
  "bosh_environment": "${BOSH_ENVIRONMENT}",
  "bosh_gw_user": "${BOSH_GW_USER}",
  "bosh_gw_host": "${BOSH_GW_HOST}",
  "bosh_gw_private_key": "${director_gw_key}"
}
EOF

cat ${CONFIG}

ginkgo -nodes=3
