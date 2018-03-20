

### Supported API Parameters For `Storm_external_standalone` Plan

Parameters:
* `ATTR_krb5conf`: content of `/etc/krb5.conf`, must be base64 encoded.
* `ATTR_kafkaclient-keytab`: content of `/etc/security/keytabs/storm.headless.keytab`, must be base64 encoded.
* `ATTR_kafkaclient-service-name`: value of `serviceName` in `storm_jaas.conf`.
* `ATTR_kafkaclient-principal`: value of `principal` in `storm_jaas.conf`.

Custom Options:
```
"customize": {
    "supervisors": {
        "default":2,
        "max":10,
        "unit":"个",
        "step":1,
        "desc":"Storm集群supervisor数量"
    },
    "memory": {
        "default":0.5,
        "max":32,
        "unit":"GB",
        "step":0.1,
        "desc":"Storm集群supervisor节点内存设置"
    },
    "workers": {
        "default":4,
        "max":30,
        "unit":"个",
        "step":1,
        "desc":"每个supervisor上的worker数量"
    }
}
```


