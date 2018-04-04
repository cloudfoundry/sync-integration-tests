#!/bin/bash

set -eu

# ENV
: "${CF_VARS_STORE:=$HOME/deployments/vbox/deployment-vars.yml}"
: "${BOSH_VARS_STORE:=$HOME/deployments/vbox/creds.yml}"

if [ ! -f "${CF_VARS_STORE}" ]; then
  echo "Unable to find CF vars store at ${CF_VARS_STORE}. Specify your vars store with \$CF_VARS_STORE!"
  exit 1
fi
if [ ! -f "${BOSH_VARS_STORE}" ]; then
  echo "Unable to find BOSH vars store at ${BOSH_VARS_STORE}. Specify your vars store with \$BOSH_VARS_STORE!"
  exit 1
fi

if [ $(uname) == 'Darwin' ]; then
  config_path=$(mktemp -d -t 'sits')
else
  config_path=$(mktemp -d)
fi
trap "{ rm -rf ${config_path}; }" EXIT

bbs_client_cert_path="${config_path}/bbs.crt"
bosh interpolate --path /cc_tls/certificate "${CF_VARS_STORE}" > "${bbs_client_cert_path}"
bbs_client_key_path="${config_path}/bbs.key"
bosh interpolate --path /cc_tls/private_key "${CF_VARS_STORE}" > "${bbs_client_key_path}"

director_ca_cert="${config_path}/director.crt"
bosh interpolate --path /director_ssl/ca "${BOSH_VARS_STORE}" > "${director_ca_cert}"
director_gw_key="${config_path}/director_gw.key"
bosh interpolate --path /jumpbox_ssh/private_key "${BOSH_VARS_STORE}" > "${director_gw_key}"
chmod 600 "${director_gw_key}"

set +e
bosh_password="$(bosh interpolate --path /admin_password "${BOSH_VARS_STORE}" 2> /dev/null)"
exit_code="$?"
set -e

if [ "${exit_code}" != 0 ]; then
  bosh_password="admin"
fi

export CONFIG=${config_path}/config.json
cat > "$CONFIG" <<EOF
{
  "cf_api": "api.bosh-lite.com",
  "cf_admin_user": "admin",
  "cf_admin_password": "admin",
  "cf_skip_ssl_validation": true,
  "cf_apps_domain": "bosh-lite.com",
  "bbs_client_cert": "${bbs_client_cert_path}",
  "bbs_client_key": "${bbs_client_key_path}",
  "bosh_ca_cert": "${director_ca_cert}",
  "bosh_client": "admin",
  "bosh_client_secret": "${bosh_password}",
  "bosh_environment": "https://192.168.50.6:25555",
  "bosh_gw_user": "jumpbox",
  "bosh_gw_host": "192.168.50.6",
  "bosh_gw_private_key": "${director_gw_key}"
}
EOF

ginkgo -nodes=3
