

### Supported API Parameters For `Storm_external_standalone` Plan

* `krb5conf`: content of `/etc/krb5.conf`, must be base64 encoded.
* `kafkaclient-keytab`: content of `/etc/security/keytabs/storm.headless.keytab`, must be base64 encoded.
* `kafkaclient-service-name`: value of `serviceName` in `storm_jaas.conf`.
* `kafkaclient-principal`: value of `principal` in `storm_jaas.conf`.




