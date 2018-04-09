

### Supported API Parameters And Custom Options For `Redis_volumes_cluster_with_replicas` and `Redis_volumes_cluster` Plans

Parameters:
* `ATTR_enable_auth`: enable auth or not. Enable if the value is "yes", "true" or "1".

Custom Options (`replicas` is only for Redis_volumes_cluster_with_replicas):
```
"customize": {
	"nodes": {
		"default":3,
		"max":5,
		"price":100000000,
		"unit":"nodes",
		"step":1,
		"desc":"Redis集群node数量"
	},
	"memory": {
		"default":0.1,
		"max":2,
		"price":10000000,
		"unit":"GB",
		"step":0.1,
		"desc":"Redis集群节点内存设置"
	},
	"volumeSize": {
		"default":1,
		"max":10,
		"price":10000000,
		"unit":"GB",
		"step":1,
		"desc":"Redis挂卷大小设置"
	},
	"replicas": {
		"default":1,
		"max":3,
		"price":100000000,
		"unit":"replicas",
		"step":1,
		"desc":"Redis集群slaves/master数量"
	}
}
```

### Supported API Parameters And Custom Options For `Storm_external_standalone` Plan

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


