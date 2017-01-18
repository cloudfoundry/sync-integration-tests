#!/bin/bash

config_path=$(mktemp -d)
export CONFIG=${config_path}/config.json

cat > "$CONFIG" <<EOF
{
  "api": "api.bosh-lite.com",
  "admin_user": "admin",
  "admin_password": "admin",
  "skip_ssl_validation": true,
  "apps_domain": "bosh-lite.com"
}
EOF

ginkgo -nodes=3 --  \
  --bbs-client-cert="fixtures/client_cert" \
  --bbs-client-key="fixtures/client_key"
