

### 数据库软件和版本

本程序使用ETCD做为存储数据库。
ETCD版本需兼容ETCD API v2。

### API接口

#### GET /v2/catalog

列出所有的支持的Service Broker列表。

输出样例：
```
{
	"services":[{
		"id":"6555DBC1-E6BC-4F0D-8948-E1B5DF6BD596",
		"name":"Storm",
		"description":"Storm为分布式实时计算框架。版本：v0.9.2",
		"bindable":true,
		"tags":["storm","openshift"],
		"plan_updateable":false,
		"plans":[{
				"id":"ef12ed9a-87f5-11e7-b949-0fc03853e5ec",
				"name":"external_standalone",
				"description":"HA Storm on Openshift exposed to external",
				"free":true,
				"metadata":{"displayName":"Shared and Free",
				"bullets":["1 GB of Disk","20 connections"],
				"customize":{
					"memory":{
						"default":0.5,
						"desc":"Storm集群supervisor节点内存设置",
						"max":32,
						"price":1e+07,
						"step":0.1,
						"unit":"GB"
					},
					"supervisors":{
						"default":2,
						"desc":"Storm集群supervisor数量",
						"max":10,
						"price":1e+08,
						"step":1,
						"unit":"个"
					},
					"workers":{
						"default":4,
						"desc":"每个supervisor上的worker数量",
						"max":30,
						"price":1e+07,
						"step":1,
						"unit":"个"
					}
				}
			}
		}],
		"metadata":{
			"displayName":"Storm",
			"imageUrl":"pub/assets/Storm.png",
			"longDescription":"Managed, highly available storm clusters in the cloud.",
			"providerDisplayName":"Asiainfo",
			"documentationUrl":"https://storm.apache.org/releases/1.0.1/index.html",
			"supportUrl":"https://storm.apache.org/"
		}
	},
	{
		"id":"A54BC117-25E3-4E41-B8F7-14FC314D04D3",
		"name":"Redis",
		"description":"A Sample Redis (v2.8) cluster on Openshift",
		"bindable":true,
		"tags":["redis","openshift"],
		"plan_updateable":false,
		"plans":[{
			"id":"94bcf092-74e7-49b1-add6-314fb2c7bdfb",
			"name":"volumes_cluster",
			"description":"Redis Cluster With Volumes on Openshift",
			"free":true,
			"metadata":{
				"displayName":"Shared and Free",
				"bullets":["1 GB of Disk","20 connections"],
				"customize":{
					"memory":{
						"default":0.1,
						"desc":"Redis集群节点内存设置",
						"max":2,
						"price":1e+07,
						"step":0.1,
						"unit":"GB"
					},
					"nodes":{
						"default":3,
						"desc":"Redis集群node数量",
						"max":10,
						"price":1e+08,
						"step":1,
						"unit":"nodes"
					},
					"replicas":{
						"default":0,
						"desc":"Redis集群slaves/node数量",
						"max":3,
						"price":1e+08,
						"step":1,
						"unit":"replicas"
					},
					"volumeSize":{
						"default":1,
						"desc":"Redis挂卷大小设置",
						"max":3,
						"price":1e+07,
						"step":1,
						"unit":"GB"
					}
				}
			}
		}],
		"metadata":{
			"displayName":
			"Redis",
			"imageUrl":"http://redis.io/images/redis-300dpi.png",
			"longDescription":"Managed, highly available redis clusters in the cloud.",
			"providerDisplayName":"Asiainfo",
			"documentationUrl":"http://redis.io/documentation",
			"supportUrl":"http://redis.io"
		}
	}]
}
```

#### PUT /v2/service_instances/{instance_id}

创建一个Service Broker Instance。

Path参数
* `instance_id`: 欲创建的Service Broker Instance ID, 最好为一个uuid。

Query参数
* `accepts_incomplete`: 是否支持异步创建。

Body参数（JSON格式）
* service_id: Service Broker Instance ID
* plan_id： 套餐ID
* organization_guid: 用户的组织ID
* space_guid: 用户的项目ID
* parameters: 其它参数，包括在Cataglog中对应Service Broker的对应plan的metadata.customize中列出的参数。

服务器回应: 所创建的Service Brokder Instance的访问信息。

curl样例：
```
curl -i -X PUT \
  -H "Content-Type: application/json" \
  http://user:password@server.addr/v2/service_instances/redis-cluster-demo \
  -d '{ \
  	"service_id":"A54BC117-25E3-4E41-B8F7-14FC314D04D3", \
  	"plan_id":"94bcf092-74e7-49b1-add6-314fb2c7bdfb", \
  	"organization_guid": "default", \
  	"space_guid":"space-guid", \
  	"parameters": {"nodes":3, "replicas":1, "memory":0.3} \
   }'
```

回应样例（JSON格式）：
```
{
	"credentials":{
		"uri":"10.1.235.179:32414, 10.1.235.179:32556, 10.1.235.179:30932",
		"password":"4a6431e1b0428226b873e3aabbdc5c61"
	}
}
```

#### PATCH /v2/service_instances/{instance_id}

修改一个Service Broker Instance。

Path参数
* `instance_id`: 欲创建的Service Broker Instance ID, 最好为一个uuid。

Query参数
* `accepts_incomplete`: 是否支持异步修改。

Body参数（JSON格式）
* service_id: Service Broker Instance ID
* plan_id： 套餐ID
* organization_guid: 用户的组织ID
* space_guid: 用户的项目ID
* parameters: 其它参数，包括在Cataglog中对应Service Broker的对应plan的metadata.customize中列出的参数。

curl样例：
```
curl -i -X PATCH \
  -H "Content-Type: application/json" \
  http://user:password@server.addr/v2/service_instances/redis-cluster-demo \
  -d '{ \
  	"service_id":"A54BC117-25E3-4E41-B8F7-14FC314D04D3", \
  	"plan_id":"94bcf092-74e7-49b1-add6-314fb2c7bdfb", \
  	"organization_guid": "default", \
  	"space_guid":"space-guid", \
  	"parameters": {"nodes":5, "replicas":2, "memory":0.6} \
   }'
```

回应样例（JSON格式）：
```
{
}
```

#### DELETE /v2/service_instances/{instance_id}

删除一个Service Broker Instance。

Path参数
* `instance_id`: 欲删除的Service Broker Instance ID。

Query参数
* `service_id`: 对应的Service Broker ID。
* `plan_id`: 对应的Service Broker Plan ID。
* `accepts_incomplete`: 是否支持异步删除。

服务器回应: 所创建的Service Brokder Instance的访问信息。

curl样例：
```
curl -i -X DELETE \
  -L 'http://user:password@server.addr/v2/service_instances/redis-cluster-demo?service_id=A54BC117-25E3-4E41-B8F7-14FC314D04D3&plan_id=94bcf092-74e7-49b1-add6-314fb2c7bdfb'
```

回应样例（JSON格式）：
```
{
}
```

#### GET /v2/service_instances/{instance_id}/last_operation

获取上一个异步操作的进度。

Path参数
* `instance_id`: 欲删除的Service Broker Instance ID。

Query参数
* `service_id`: 对应的Service Broker ID。
* `plan_id`: 对应的Service Broker Plan ID。

服务器回应: 
* `state`: 当前状态。("in progress" | "succeeded" | "failed")
* `description`: 可选。状态描述。

curl样例：
```
curl -i -X DELETE \
  -L 'http://user:password@server.addr/v2/service_instances/redis-cluster-demo?service_id=A54BC117-25E3-4E41-B8F7-14FC314D04D3&plan_id=94bcf092-74e7-49b1-add6-314fb2c7bdfb'
```

回应样例（JSON格式）：
```
{
	"state": "succeeded",
	"description": "succeeded"
}
```

### 支持的Backing Services和它们的Plans

[详见etcd registry](registery/etcdinit.sh)

### 需要的环境变量

Open Shift集群访问凭证：
* OPENSHIFTADDR
* OPENSHIFTUSER
* OPENSHIFTPASS

生成的Backing Service Instance资源所在项目。
* SBNAMESPACE

Service Broker Instance 泛域名:
* ENDPOINTSUFFIX

Open Shift nodes IP addresses和域名:
* NODE_ADDRESSES
* NODE_DOMAINS

外部ZooKeeper服务器地址:
* EXTERNALZOOKEEPERSERVERS

OCSP相关
* OCSP_OCM
* OCSP_OCM_PORT
* OCSP_HDP_VERSION

Service Broker Instance Pod Images
* ANACONDAIMAGE: Anaconda 镜像
* CASSANDRAIMAGE: Cassandra 镜像
* DATAIKUIMAGE: Dataiku 镜像
* ELASTICSEARCHVOLUMEIMAGE: Elastics Search 镜像(支持挂卷)
* ETCDIMAGE: ETCD 镜像
* ETCDBOOTIMAGE: ETCD seed pod 镜像
* ETCDVOLUMEIMAGE: ETCD 镜像(支持挂卷)
* KAFKAIMAGE: Kafka 镜像
* KAFKAVOLUMEIMAGE: Kafka 镜像(支持挂卷)
* KETTLEIMAGE: Kettle 镜像
* MONGOVOLUMEIMAGE: MongoDB 镜像(支持挂卷)
* NEO4JVOLUMEIMAGE: Neo4j 镜像(支持挂卷)
* NIFIIMAGE: Nifi 镜像
* OCSPIMAGE: OCSP 镜像
* PYSPIDERIMAGE: PySpider 镜像
* RABBITMQIMAGE: RabbitMQ 镜像
* REDISIMAGE: Redis高可用（带哨兵）镜像
* REDIS32IMAGE: Redis 单Master镜像
* REDISCLUSTERIMAGE: Redis分布式Cluster镜像
* REDISCLUSTERTRIBIMAGE: Redis Cluster trib.rb 镜像 （已不再需要）
* SIMPLEFILEUPLOADERIMAGE: Simple File Uploader 镜像
* STORMIMAGE: Storm 镜像
* STORMEXTERNALIMAGE: Storm With External ZooKeeper 镜像
* SPARKIMAGE: Spark 镜像
* TENSORFLOWIMAGE: TensorFlow 镜像
* ZEPPLINIMAGE: Zepplin 镜像
* ZOOKEEPERIMAGE: ZooKeeper 镜像
* ZOOKEEPEREXHIBITORIMAGE: ZooKeeper Exhibitor 镜像
* MARIADBIMAGE, PHPMYADMINIMAGE, PROMETHEUSMYSQLEXPORTERIMAGE: MySQL Maria高可用多Master集群所需镜像。
* ESCLUSTERIMAGE: Elastic集群镜像。

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


