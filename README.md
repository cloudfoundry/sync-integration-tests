# Sync Integration Tests

This is a set of tests to test the sync functionality between Cloud Controller
and Diego. It forces Cloud Controller and Diego into different states so that
the sync process has to reconcile them.

To run locally, modify /etc/hosts to point bbs.service.cf.internal to the address
of your database VM in the Diego deployment (usually 10.244.16.2).

You also need a config JSON file path set at the `CONFIG` environment variable.
An example that can be used for bosh-lite is:

```json
{
  "api": "api.bosh-lite.com",
  "apps_domain": "bosh-lite.com",
  "admin_user": "admin",
  "admin_password": "admin",
  "backend": "diego",
  "default_timeout": 60,
  "name_prefix": "SITS",
  "skip_ssl_validation": true,
  "use_http": false
}
```
