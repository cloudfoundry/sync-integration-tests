# Sync Integration Tests

![i_sits](http://i2.kym-cdn.com/photos/images/original/000/264/092/e3f.jpg)

## Description

This is a set of tests to test the sync functionality between Cloud Controller
and Diego. It forces Cloud Controller and Diego into different states so that
the sync process has to reconcile them.

This test suite covers both long running processes (lrps) and Tasks. There are
fundamental differences on how lrps and tasks are handled by the syncing
processes. This difference implies that the way to create situations where
Diego and Cloud Controller go out-of-sync is very different between lrps and
tasks.

# Config and Run

## Running against a bosh-lite deployed with bosh-deployment

```bash
./run-bosh-lite.sh
```

## Running with manual configuration

You need a config JSON file path set at the `CONFIG` environment variable.
An example that can be used for bosh-lite is:

```bash
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
  "bosh_binary": "bosh",
  "bosh_api_instance": "api",
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
```

Here's how you would manually extract settings and credentials, assuming cf was
deployed with the option `--vars-store ~/deployments/vbox/deployment-vars.yml`:

```bash
cd ~/deployments/vbox
bosh interpolate --path /admin_password creds.yml >bosh_client_secret
bosh interpolate --path /cc_tls/certificate deployment-vars.yml >cc_client.crt
bosh interpolate --path /cc_tls/private_key deployment-vars.yml >cc_client.key
bosh interpolate --path /jumpbox_ssh/private_key creds.yml >bosh_gw_private_key
```

Once the file `config.json` is ready, let it roll...

```bash
CONFIG=config.json ginkgo -nodes=3
```

# Adding dependencies

During development of test cases, if you need to add a new Golang dependency,
make sure to add it to the vendor directory using `godep`.

```bash
go get <your new package> // If the dependency isn't already on your $GOPATH
godep restore
godep save ./...
```
