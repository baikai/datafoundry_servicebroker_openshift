#!/bin/bash

ETCD_USER=xxx
ETCD_PASSWORD=xxx
ETCD_ADDRESS=http://xxx:2379

API_NAME=xxx
API_PASSWORD=xxx

export ETCDCTL="etcdctl --timeout 15s --total-timeout 30s --endpoints $ETCD_ADDRESS --username $ETCD_USER:$ETCD_PASSWORD"

$ETCDCTL mkdir /servicebroker
$ETCDCTL mkdir /servicebroker/openshift
$ETCDCTL set /servicebroker/openshift/username $API_NAME
$ETCDCTL set /servicebroker/openshift/password $API_PASSWORD

$ETCDCTL mkdir /servicebroker/openshift/instance

$ETCDCTL mkdir /servicebroker/openshift/catalog




###创建服务 etcd
$ETCDCTL mkdir /servicebroker/openshift/catalog/5E397661-1385-464A-8DB7-9C4DF8CC0662 #服务id

###创建服务级的配置
$ETCDCTL set /servicebroker/openshift/catalog/5E397661-1385-464A-8DB7-9C4DF8CC0662/name "ETCD"
$ETCDCTL set /servicebroker/openshift/catalog/5E397661-1385-464A-8DB7-9C4DF8CC0662/description "ETCD是一个高可用的键值存储系统,主要用于共享配置和服务发现。版本：v2.3.0"
$ETCDCTL set /servicebroker/openshift/catalog/5E397661-1385-464A-8DB7-9C4DF8CC0662/bindable true
$ETCDCTL set /servicebroker/openshift/catalog/5E397661-1385-464A-8DB7-9C4DF8CC0662/planupdatable false
$ETCDCTL set /servicebroker/openshift/catalog/5E397661-1385-464A-8DB7-9C4DF8CC0662/tags 'etcd,openshift'
$ETCDCTL set /servicebroker/openshift/catalog/5E397661-1385-464A-8DB7-9C4DF8CC0662/metadata '{"displayName":"ETCD","imageUrl":"pub/assets/ETCD.png","longDescription":"Managed, highly available etcd clusters in the cloud.","providerDisplayName":"Asiainfo","documentationUrl":"https://coreos.com/etcd/docs/latest","supportUrl":"https://coreos.com"}'

###创建套餐目录
$ETCDCTL mkdir /servicebroker/openshift/catalog/5E397661-1385-464A-8DB7-9C4DF8CC0662/plan
###创建套餐1
$ETCDCTL mkdir /servicebroker/openshift/catalog/5E397661-1385-464A-8DB7-9C4DF8CC0662/plan/204F8288-F8D9-4806-8661-EB48D94504B3
$ETCDCTL set /servicebroker/openshift/catalog/5E397661-1385-464A-8DB7-9C4DF8CC0662/plan/204F8288-F8D9-4806-8661-EB48D94504B3/name "standalone"
$ETCDCTL set /servicebroker/openshift/catalog/5E397661-1385-464A-8DB7-9C4DF8CC0662/plan/204F8288-F8D9-4806-8661-EB48D94504B3/description "单独ETCD实例"
$ETCDCTL set /servicebroker/openshift/catalog/5E397661-1385-464A-8DB7-9C4DF8CC0662/plan/204F8288-F8D9-4806-8661-EB48D94504B3/metadata '{"bullets":["1 GB of Disk","20 connections"],"displayName":"Shared and Free" }'
$ETCDCTL set /servicebroker/openshift/catalog/5E397661-1385-464A-8DB7-9C4DF8CC0662/plan/204F8288-F8D9-4806-8661-EB48D94504B3/free true
###创建套餐目录2(pvc)
$ETCDCTL mkdir /servicebroker/openshift/catalog/5E397661-1385-464A-8DB7-9C4DF8CC0662/plan/256D56C0-B83D-11E6-B227-2714EF851DCA
$ETCDCTL set /servicebroker/openshift/catalog/5E397661-1385-464A-8DB7-9C4DF8CC0662/plan/256D56C0-B83D-11E6-B227-2714EF851DCA/name "volumes_standalone"
$ETCDCTL set /servicebroker/openshift/catalog/5E397661-1385-464A-8DB7-9C4DF8CC0662/plan/256D56C0-B83D-11E6-B227-2714EF851DCA/description "HA etcd on Openshift"
$ETCDCTL set /servicebroker/openshift/catalog/5E397661-1385-464A-8DB7-9C4DF8CC0662/plan/256D56C0-B83D-11E6-B227-2714EF851DCA/metadata '{"bullets":["1 GB of Disk","20 connections"],"displayName":"Shared and Free" }'
$ETCDCTL set /servicebroker/openshift/catalog/5E397661-1385-464A-8DB7-9C4DF8CC0662/plan/256D56C0-B83D-11E6-B227-2714EF851DCA/free true




###创建服务 spark
$ETCDCTL mkdir /servicebroker/openshift/catalog/0674A0E3-5EE4-425C-BE43-5A61EB3F52A6 #服务id

###创建服务级的配置
$ETCDCTL set /servicebroker/openshift/catalog/0674A0E3-5EE4-425C-BE43-5A61EB3F52A6/name "Spark"
$ETCDCTL set /servicebroker/openshift/catalog/0674A0E3-5EE4-425C-BE43-5A61EB3F52A6/description "A Sample Spark (v1.5.2) cluster on Openshift with one worker node"
$ETCDCTL set /servicebroker/openshift/catalog/0674A0E3-5EE4-425C-BE43-5A61EB3F52A6/bindable true
$ETCDCTL set /servicebroker/openshift/catalog/0674A0E3-5EE4-425C-BE43-5A61EB3F52A6/planupdatable false
$ETCDCTL set /servicebroker/openshift/catalog/0674A0E3-5EE4-425C-BE43-5A61EB3F52A6/tags 'spark,openshift'
$ETCDCTL set /servicebroker/openshift/catalog/0674A0E3-5EE4-425C-BE43-5A61EB3F52A6/metadata '{"displayName":"Spark","imageUrl":"http://spark.apache.org/images/spark-logo-trademark.png","longDescription":"Managed, highly available Spark clusters in the cloud.","providerDisplayName":"Asiainfo","documentationUrl":"http://spark.apache.org/docs/latest/","supportUrl":"http://spark.apache.org"}'

###创建套餐目录
$ETCDCTL mkdir /servicebroker/openshift/catalog/0674A0E3-5EE4-425C-BE43-5A61EB3F52A6/plan
###创建套餐1
$ETCDCTL mkdir /servicebroker/openshift/catalog/0674A0E3-5EE4-425C-BE43-5A61EB3F52A6/plan/CB00754C-11FF-444F-8419-9AA9B1E04970
$ETCDCTL set /servicebroker/openshift/catalog/0674A0E3-5EE4-425C-BE43-5A61EB3F52A6/plan/CB00754C-11FF-444F-8419-9AA9B1E04970/name "One_Worker"
$ETCDCTL set /servicebroker/openshift/catalog/0674A0E3-5EE4-425C-BE43-5A61EB3F52A6/plan/CB00754C-11FF-444F-8419-9AA9B1E04970/description "Spark on Openshift"
$ETCDCTL set /servicebroker/openshift/catalog/0674A0E3-5EE4-425C-BE43-5A61EB3F52A6/plan/CB00754C-11FF-444F-8419-9AA9B1E04970/metadata '{"bullets":["1 GB of Disk","one worker node"],"displayName":"Free" }'
$ETCDCTL set /servicebroker/openshift/catalog/0674A0E3-5EE4-425C-BE43-5A61EB3F52A6/plan/CB00754C-11FF-444F-8419-9AA9B1E04970/free true
###创建套餐2
$ETCDCTL mkdir /servicebroker/openshift/catalog/0674A0E3-5EE4-425C-BE43-5A61EB3F52A6/plan/65242C9B-C266-4F1D-A28B-98B1A2043A84
$ETCDCTL set /servicebroker/openshift/catalog/0674A0E3-5EE4-425C-BE43-5A61EB3F52A6/plan/65242C9B-C266-4F1D-A28B-98B1A2043A84/name "Three_Workers"
$ETCDCTL set /servicebroker/openshift/catalog/0674A0E3-5EE4-425C-BE43-5A61EB3F52A6/plan/65242C9B-C266-4F1D-A28B-98B1A2043A84/description "HA Spark on Openshift"
$ETCDCTL set /servicebroker/openshift/catalog/0674A0E3-5EE4-425C-BE43-5A61EB3F52A6/plan/65242C9B-C266-4F1D-A28B-98B1A2043A84/metadata '{"bullets":["1 GB of Disk","three worker nodes"],"displayName":"High Available" }'
$ETCDCTL set /servicebroker/openshift/catalog/0674A0E3-5EE4-425C-BE43-5A61EB3F52A6/plan/65242C9B-C266-4F1D-A28B-98B1A2043A84/free false




###创建服务 zookeeper
$ETCDCTL mkdir /servicebroker/openshift/catalog/FA780A47-4AB2-4035-8638-287538B13416 #服务id

###创建服务级的配置
$ETCDCTL set /servicebroker/openshift/catalog/FA780A47-4AB2-4035-8638-287538B13416/name "ZooKeeper"
$ETCDCTL set /servicebroker/openshift/catalog/FA780A47-4AB2-4035-8638-287538B13416/description "ZooKeeper是一个分布式的，开放源码的分布式应用程序协调服务。版本：v3.4.8"
$ETCDCTL set /servicebroker/openshift/catalog/FA780A47-4AB2-4035-8638-287538B13416/bindable true
$ETCDCTL set /servicebroker/openshift/catalog/FA780A47-4AB2-4035-8638-287538B13416/planupdatable false
$ETCDCTL set /servicebroker/openshift/catalog/FA780A47-4AB2-4035-8638-287538B13416/tags 'zookeeper,openshift'
$ETCDCTL set /servicebroker/openshift/catalog/FA780A47-4AB2-4035-8638-287538B13416/metadata '{"displayName":"ZooKeeper","imageUrl":"pub/assets/ZooKeeper.png","longDescription":"Managed, highly available zookeeper clusters in the cloud.","providerDisplayName":"Asiainfo","documentationUrl":"https://zookeeper.apache.org/doc/trunk","supportUrl":"https://zookeeper.apache.org/"}'

###创建套餐目录
$ETCDCTL mkdir /servicebroker/openshift/catalog/FA780A47-4AB2-4035-8638-287538B13416/plan
###创建套餐1 (emptyDir)
$ETCDCTL mkdir /servicebroker/openshift/catalog/FA780A47-4AB2-4035-8638-287538B13416/plan/221F1338-96C1-4135-A865-9CDA4C12A185
$ETCDCTL set /servicebroker/openshift/catalog/FA780A47-4AB2-4035-8638-287538B13416/plan/221F1338-96C1-4135-A865-9CDA4C12A185/name "standalone"
$ETCDCTL set /servicebroker/openshift/catalog/FA780A47-4AB2-4035-8638-287538B13416/plan/221F1338-96C1-4135-A865-9CDA4C12A185/description "单独zookeeper实例"
$ETCDCTL set /servicebroker/openshift/catalog/FA780A47-4AB2-4035-8638-287538B13416/plan/221F1338-96C1-4135-A865-9CDA4C12A185/metadata '{"bullets":["1 GB of Disk","20 connections"],"displayName":"Shared and Free" }'
$ETCDCTL set /servicebroker/openshift/catalog/FA780A47-4AB2-4035-8638-287538B13416/plan/221F1338-96C1-4135-A865-9CDA4C12A185/free true
###创建套餐2 (pvc)
$ETCDCTL mkdir /servicebroker/openshift/catalog/FA780A47-4AB2-4035-8638-287538B13416/plan/dffc3555-eb18-4c7b-ac56-bd326b19dcd0
$ETCDCTL set /servicebroker/openshift/catalog/FA780A47-4AB2-4035-8638-287538B13416/plan/dffc3555-eb18-4c7b-ac56-bd326b19dcd0/name "volumes_standalone"
$ETCDCTL set /servicebroker/openshift/catalog/FA780A47-4AB2-4035-8638-287538B13416/plan/dffc3555-eb18-4c7b-ac56-bd326b19dcd0/description "HA ZooKeeper With Volumes on Openshift"
$ETCDCTL set /servicebroker/openshift/catalog/FA780A47-4AB2-4035-8638-287538B13416/plan/dffc3555-eb18-4c7b-ac56-bd326b19dcd0/metadata '{"bullets":["1 GB of Disk","20 connections"],"displayName":"Shared and Free" }'
$ETCDCTL set /servicebroker/openshift/catalog/FA780A47-4AB2-4035-8638-287538B13416/plan/dffc3555-eb18-4c7b-ac56-bd326b19dcd0/free true


###创建服务 RabbitMQ
$ETCDCTL mkdir /servicebroker/openshift/catalog/86272DCB-86D5-4039-9E05-894436B8E71D #服务id

###创建服务级的配置
$ETCDCTL set /servicebroker/openshift/catalog/86272DCB-86D5-4039-9E05-894436B8E71D/name "RabbitMQ"
$ETCDCTL set /servicebroker/openshift/catalog/86272DCB-86D5-4039-9E05-894436B8E71D/description "RabbitMQ是流行的开源消息队列系统。版本v3.6.1"
$ETCDCTL set /servicebroker/openshift/catalog/86272DCB-86D5-4039-9E05-894436B8E71D/bindable true
$ETCDCTL set /servicebroker/openshift/catalog/86272DCB-86D5-4039-9E05-894436B8E71D/planupdatable false
$ETCDCTL set /servicebroker/openshift/catalog/86272DCB-86D5-4039-9E05-894436B8E71D/tags 'rabbitmq,openshift'
$ETCDCTL set /servicebroker/openshift/catalog/86272DCB-86D5-4039-9E05-894436B8E71D/metadata '{"displayName":"RabbitMQ","imageUrl":"pub/assets/RabbitMQ.png","longDescription":"Managed, highly available rabbitmq clusters in the cloud.","providerDisplayName":"Asiainfo","documentationUrl":"https://www.rabbitmq.com/documentation.html","supportUrl":"https://www.rabbitmq.com/"}'

###创建套餐目录
$ETCDCTL mkdir /servicebroker/openshift/catalog/86272DCB-86D5-4039-9E05-894436B8E71D/plan
###创建套餐1
$ETCDCTL mkdir /servicebroker/openshift/catalog/86272DCB-86D5-4039-9E05-894436B8E71D/plan/CC5ED8A2-1677-43A0-ADE5-27F713C6F51B
$ETCDCTL set /servicebroker/openshift/catalog/86272DCB-86D5-4039-9E05-894436B8E71D/plan/CC5ED8A2-1677-43A0-ADE5-27F713C6F51B/name "standalone"
$ETCDCTL set /servicebroker/openshift/catalog/86272DCB-86D5-4039-9E05-894436B8E71D/plan/CC5ED8A2-1677-43A0-ADE5-27F713C6F51B/description "单独rabbitMQ实例"
$ETCDCTL set /servicebroker/openshift/catalog/86272DCB-86D5-4039-9E05-894436B8E71D/plan/CC5ED8A2-1677-43A0-ADE5-27F713C6F51B/metadata '{"bullets":["1 GB of Disk","20 connections"],"displayName":"Shared and Free" }'
$ETCDCTL set /servicebroker/openshift/catalog/86272DCB-86D5-4039-9E05-894436B8E71D/plan/CC5ED8A2-1677-43A0-ADE5-27F713C6F51B/free true
###创建套餐2(pvc)
$ETCDCTL mkdir /servicebroker/openshift/catalog/86272DCB-86D5-4039-9E05-894436B8E71D/plan/28749ee2-6f30-4967-8311-2bf34f9a5421
$ETCDCTL set /servicebroker/openshift/catalog/86272DCB-86D5-4039-9E05-894436B8E71D/plan/28749ee2-6f30-4967-8311-2bf34f9a5421/name "volumes_standalone"
$ETCDCTL set /servicebroker/openshift/catalog/86272DCB-86D5-4039-9E05-894436B8E71D/plan/28749ee2-6f30-4967-8311-2bf34f9a5421/description "HA RabbitMQ With Volumes on Openshift"
$ETCDCTL set /servicebroker/openshift/catalog/86272DCB-86D5-4039-9E05-894436B8E71D/plan/28749ee2-6f30-4967-8311-2bf34f9a5421/metadata '{"bullets":["1 GB of Disk","20 connections"],"displayName":"Shared and Free" }'
$ETCDCTL set /servicebroker/openshift/catalog/86272DCB-86D5-4039-9E05-894436B8E71D/plan/28749ee2-6f30-4967-8311-2bf34f9a5421/free true






###创建服务 Redis
$ETCDCTL mkdir /servicebroker/openshift/catalog/A54BC117-25E3-4E41-B8F7-14FC314D04D3 #服务id

###创建服务级的配置
$ETCDCTL set /servicebroker/openshift/catalog/A54BC117-25E3-4E41-B8F7-14FC314D04D3/name "Redis"
$ETCDCTL set /servicebroker/openshift/catalog/A54BC117-25E3-4E41-B8F7-14FC314D04D3/description "Redis是一个可基于内存亦可持久化的日志型、Key-Value数据库。版本：v2.8"
$ETCDCTL set /servicebroker/openshift/catalog/A54BC117-25E3-4E41-B8F7-14FC314D04D3/bindable true
$ETCDCTL set /servicebroker/openshift/catalog/A54BC117-25E3-4E41-B8F7-14FC314D04D3/planupdatable false
$ETCDCTL set /servicebroker/openshift/catalog/A54BC117-25E3-4E41-B8F7-14FC314D04D3/tags 'redis,openshift'
$ETCDCTL set /servicebroker/openshift/catalog/A54BC117-25E3-4E41-B8F7-14FC314D04D3/metadata '{"displayName":"Redis","imageUrl":"pub/assets/Redis.png","longDescription":"Managed, highly available redis clusters in the cloud.","providerDisplayName":"Asiainfo","documentationUrl":"http://redis.io/documentation","supportUrl":"http://redis.io"}'

###创建套餐目录
$ETCDCTL mkdir /servicebroker/openshift/catalog/A54BC117-25E3-4E41-B8F7-14FC314D04D3/plan
###创建套餐1 (emptyDir)
#$ETCDCTL mkdir /servicebroker/openshift/catalog/A54BC117-25E3-4E41-B8F7-14FC314D04D3/plan/46ED475B-82A7-4C46-9767-0E3E806848F5
#$ETCDCTL set /servicebroker/openshift/catalog/A54BC117-25E3-4E41-B8F7-14FC314D04D3/plan/46ED475B-82A7-4C46-9767-0E3E806848F5/name "standalone"
#$ETCDCTL set /servicebroker/openshift/catalog/A54BC117-25E3-4E41-B8F7-14FC314D04D3/plan/46ED475B-82A7-4C46-9767-0E3E806848F5/description "单独Redis实例"
#$ETCDCTL set /servicebroker/openshift/catalog/A54BC117-25E3-4E41-B8F7-14FC314D04D3/plan/46ED475B-82A7-4C46-9767-0E3E806848F5/metadata '{"bullets":["1 GB of Disk","20 connections"],"displayName":"Shared and Free" }'
#$ETCDCTL set /servicebroker/openshift/catalog/A54BC117-25E3-4E41-B8F7-14FC314D04D3/plan/46ED475B-82A7-4C46-9767-0E3E806848F5/free true
###创建套餐2 (pvc)
#$ETCDCTL mkdir /servicebroker/openshift/catalog/A54BC117-25E3-4E41-B8F7-14FC314D04D3/plan/f8554827-4f67-4082-84af-6d35475c1ea8
#$ETCDCTL set /servicebroker/openshift/catalog/A54BC117-25E3-4E41-B8F7-14FC314D04D3/plan/f8554827-4f67-4082-84af-6d35475c1ea8/name "volumes_standalone"
#$ETCDCTL set /servicebroker/openshift/catalog/A54BC117-25E3-4E41-B8F7-14FC314D04D3/plan/f8554827-4f67-4082-84af-6d35475c1ea8/description "HA Redis With Volumes on Openshift"
#$ETCDCTL set /servicebroker/openshift/catalog/A54BC117-25E3-4E41-B8F7-14FC314D04D3/plan/f8554827-4f67-4082-84af-6d35475c1ea8/metadata '{"bullets":["1 GB of Disk","20 connections"],"displayName":"Shared and Free" }'
#$ETCDCTL set /servicebroker/openshift/catalog/A54BC117-25E3-4E41-B8F7-14FC314D04D3/plan/f8554827-4f67-4082-84af-6d35475c1ea8/free true
###创建套餐3 (single master pvc)
$ETCDCTL mkdir /servicebroker/openshift/catalog/33fe30bd-9c31-40aa-b853-598c23bbb0da/plan/6c97104b-2d7d-44f9-a053-5c8d018d25a6
$ETCDCTL set /servicebroker/openshift/catalog/33fe30bd-9c31-40aa-b853-598c23bbb0da/plan/6c97104b-2d7d-44f9-a053-5c8d018d25a6/name "volumes_single"
$ETCDCTL set /servicebroker/openshift/catalog/33fe30bd-9c31-40aa-b853-598c23bbb0da/plan/6c97104b-2d7d-44f9-a053-5c8d018d25a6/description "Redis Single Master With Volumes on Openshift"
$ETCDCTL set /servicebroker/openshift/catalog/33fe30bd-9c31-40aa-b853-598c23bbb0da/plan/6c97104b-2d7d-44f9-a053-5c8d018d25a6/metadata '{"bullets":["1 GB of Disk","20 connections"],"displayName":"Shared and Free" }'
$ETCDCTL set /servicebroker/openshift/catalog/33fe30bd-9c31-40aa-b853-598c23bbb0da/plan/6c97104b-2d7d-44f9-a053-5c8d018d25a6/free true
###创建套餐4 (4.0+ cluster)
$ETCDCTL mkdir /servicebroker/openshift/catalog/A54BC117-25E3-4E41-B8F7-14FC314D04D3/plan/94bcf092-74e7-49b1-add6-314fb2c7bdfb
$ETCDCTL set /servicebroker/openshift/catalog/A54BC117-25E3-4E41-B8F7-14FC314D04D3/plan/94bcf092-74e7-49b1-add6-314fb2c7bdfb/name "volumes_cluster"
$ETCDCTL set /servicebroker/openshift/catalog/A54BC117-25E3-4E41-B8F7-14FC314D04D3/plan/94bcf092-74e7-49b1-add6-314fb2c7bdfb/description "Redis Cluster With Volumes on Openshift"
$ETCDCTL set /servicebroker/openshift/catalog/A54BC117-25E3-4E41-B8F7-14FC314D04D3/plan/94bcf092-74e7-49b1-add6-314fb2c7bdfb/metadata '{"bullets":["1 GB of Disk","20 connections"],"displayName":"Shared and Free","customize":{"nodes":{"default":3,"max":5,"price":100000000,"unit":"nodes","step":1,"desc":"Redis集群node数量"},"memory":{"default":0.1,"max":2,"price":10000000,"unit":"GB","step":0.1,"desc":"Redis集群节点内存设置"},"volumeSize":{"default":1,"max":10,"price":10000000,"unit":"GB","step":1,"desc":"Redis挂卷大小设置"}}}'
$ETCDCTL set /servicebroker/openshift/catalog/A54BC117-25E3-4E41-B8F7-14FC314D04D3/plan/94bcf092-74e7-49b1-add6-314fb2c7bdfb/free true
###创建套餐5 (4.0+ cluster with replicas)
$ETCDCTL mkdir /servicebroker/openshift/catalog/A54BC117-25E3-4E41-B8F7-14FC314D04D3/plan/67117486-934f-4f94-bd2d-179ec6e309b4
$ETCDCTL set /servicebroker/openshift/catalog/A54BC117-25E3-4E41-B8F7-14FC314D04D3/plan/67117486-934f-4f94-bd2d-179ec6e309b4/name "volumes_cluster_with_replicas"
$ETCDCTL set /servicebroker/openshift/catalog/A54BC117-25E3-4E41-B8F7-14FC314D04D3/plan/67117486-934f-4f94-bd2d-179ec6e309b4/description "Redis Cluster Supporting Replicas With Volumes on Openshift"
$ETCDCTL set /servicebroker/openshift/catalog/A54BC117-25E3-4E41-B8F7-14FC314D04D3/plan/67117486-934f-4f94-bd2d-179ec6e309b4/metadata '{"bullets":["1 GB of Disk","20 connections"],"displayName":"Shared and Free","customize":{"nodes":{"default":3,"max":5,"price":100000000,"unit":"nodes","step":1,"desc":"Redis集群master node数量"},"replicas":{"default":1,"max":3,"price":100000000,"unit":"replicas","step":1,"desc":"Redis集群slaves/master数量"},"memory":{"default":0.1,"max":2,"price":10000000,"unit":"GB","step":0.1,"desc":"Redis集群节点内存设置"},"volumeSize":{"default":1,"max":10,"price":10000000,"unit":"GB","step":1,"desc":"Redis挂卷大小设置"}}}'
$ETCDCTL set /servicebroker/openshift/catalog/A54BC117-25E3-4E41-B8F7-14FC314D04D3/plan/67117486-934f-4f94-bd2d-179ec6e309b4/free true



###创建服务 Kafka
$ETCDCTL mkdir /servicebroker/openshift/catalog/9972923D-0787-4271-839C-D000BD87E309 #服务id

###创建服务级的配置
$ETCDCTL set /servicebroker/openshift/catalog/9972923D-0787-4271-839C-D000BD87E309/name "Kafka"
$ETCDCTL set /servicebroker/openshift/catalog/9972923D-0787-4271-839C-D000BD87E309/description "Kafka是一种高吞吐量的分布式发布订阅消息系统。版本：v0.9.0"
$ETCDCTL set /servicebroker/openshift/catalog/9972923D-0787-4271-839C-D000BD87E309/bindable true
$ETCDCTL set /servicebroker/openshift/catalog/9972923D-0787-4271-839C-D000BD87E309/planupdatable false
$ETCDCTL set /servicebroker/openshift/catalog/9972923D-0787-4271-839C-D000BD87E309/tags 'kafka,openshift'
$ETCDCTL set /servicebroker/openshift/catalog/9972923D-0787-4271-839C-D000BD87E309/metadata '{"displayName":"Kafka","imageUrl":"pub/assets/Kafka.png","longDescription":"Managed, highly available kafka clusters in the cloud.","providerDisplayName":"Asiainfo","documentationUrl":"http://kafka.apache.org/documentation.html","supportUrl":"http://kafka.apache.org"}'

###创建套餐目录
$ETCDCTL mkdir /servicebroker/openshift/catalog/9972923D-0787-4271-839C-D000BD87E309/plan
###创建套餐1
$ETCDCTL mkdir /servicebroker/openshift/catalog/9972923D-0787-4271-839C-D000BD87E309/plan/9A14FEF4-FB41-4C84-A175-A80492A50875
$ETCDCTL set /servicebroker/openshift/catalog/9972923D-0787-4271-839C-D000BD87E309/plan/9A14FEF4-FB41-4C84-A175-A80492A50875/name "standalone"
$ETCDCTL set /servicebroker/openshift/catalog/9972923D-0787-4271-839C-D000BD87E309/plan/9A14FEF4-FB41-4C84-A175-A80492A50875/description "单独kafka实例"
$ETCDCTL set /servicebroker/openshift/catalog/9972923D-0787-4271-839C-D000BD87E309/plan/9A14FEF4-FB41-4C84-A175-A80492A50875/metadata '{"bullets":["1 GB of Disk","20 connections"],"displayName":"Shared and Free" }'
$ETCDCTL set /servicebroker/openshift/catalog/9972923D-0787-4271-839C-D000BD87E309/plan/9A14FEF4-FB41-4C84-A175-A80492A50875/free true
###创建套餐2(pvc)
#$ETCDCTL mkdir /servicebroker/openshift/catalog/9972923D-0787-4271-839C-D000BD87E309/plan/EDCB798A-C03F-11E6-8220-AB033AF07E38
#$ETCDCTL set /servicebroker/openshift/catalog/9972923D-0787-4271-839C-D000BD87E309/plan/EDCB798A-C03F-11E6-8220-AB033AF07E38/name "volumes_standalone"
#$ETCDCTL set /servicebroker/openshift/catalog/9972923D-0787-4271-839C-D000BD87E309/plan/EDCB798A-C03F-11E6-8220-AB033AF07E38/description "HA Kafka on Openshift"
#$ETCDCTL set /servicebroker/openshift/catalog/9972923D-0787-4271-839C-D000BD87E309/plan/EDCB798A-C03F-11E6-8220-AB033AF07E38/metadata '{"bullets":["1 GB of Disk","20 connections"],"displayName":"Shared and Free" }'
#$ETCDCTL set /servicebroker/openshift/catalog/9972923D-0787-4271-839C-D000BD87E309/plan/EDCB798A-C03F-11E6-8220-AB033AF07E38/free false



###创建服务 Cassandra
#$ETCDCTL mkdir /servicebroker/openshift/catalog/3D7D7D07-D704-4B22-B492-EE5AE5301A55 #服务id

###创建服务级的配置
#$ETCDCTL set /servicebroker/openshift/catalog/3D7D7D07-D704-4B22-B492-EE5AE5301A55/name "Cassandra"
#$ETCDCTL set /servicebroker/openshift/catalog/3D7D7D07-D704-4B22-B492-EE5AE5301A55/description "Cassandra是一套开源分布式NoSQL数据库系统。版本：v3.4"
#$ETCDCTL set /servicebroker/openshift/catalog/3D7D7D07-D704-4B22-B492-EE5AE5301A55/bindable true
#$ETCDCTL set /servicebroker/openshift/catalog/3D7D7D07-D704-4B22-B492-EE5AE5301A55/planupdatable false
#$ETCDCTL set /servicebroker/openshift/catalog/3D7D7D07-D704-4B22-B492-EE5AE5301A55/tags 'cassandra,openshift'
#$ETCDCTL set /servicebroker/openshift/catalog/3D7D7D07-D704-4B22-B492-EE5AE5301A55/metadata '{"displayName":"Cassandra","imageUrl":"pub/assets/Cassandra.png","longDescription":"Managed, highly available cassandra clusters in the cloud.","providerDisplayName":"Asiainfo","documentationUrl":"https://wiki.apache.org/cassandra/GettingStarted","supportUrl":"https://cassandra.apache.org/"}'

###创建套餐目录
#$ETCDCTL mkdir /servicebroker/openshift/catalog/3D7D7D07-D704-4B22-B492-EE5AE5301A55/plan
###创建套餐1
#$ETCDCTL mkdir /servicebroker/openshift/catalog/3D7D7D07-D704-4B22-B492-EE5AE5301A55/plan/7B7EC041-2090-4ACB-AE0F-E8BDF315A778
#$ETCDCTL set /servicebroker/openshift/catalog/3D7D7D07-D704-4B22-B492-EE5AE5301A55/plan/7B7EC041-2090-4ACB-AE0F-E8BDF315A778/name "standalone"
#$ETCDCTL set /servicebroker/openshift/catalog/3D7D7D07-D704-4B22-B492-EE5AE5301A55/plan/7B7EC041-2090-4ACB-AE0F-E8BDF315A778/description "单独Cassandra实例"
#$ETCDCTL set /servicebroker/openshift/catalog/3D7D7D07-D704-4B22-B492-EE5AE5301A55/plan/7B7EC041-2090-4ACB-AE0F-E8BDF315A778/metadata '{"bullets":["1 GB of Disk","20 connections"],"displayName":"Shared and Free" }'
#$ETCDCTL set /servicebroker/openshift/catalog/3D7D7D07-D704-4B22-B492-EE5AE5301A55/plan/7B7EC041-2090-4ACB-AE0F-E8BDF315A778/free true



###创建服务 Storm
$ETCDCTL mkdir /servicebroker/openshift/catalog/6555DBC1-E6BC-4F0D-8948-E1B5DF6BD596 #服务id

###创建服务级的配置
$ETCDCTL set /servicebroker/openshift/catalog/6555DBC1-E6BC-4F0D-8948-E1B5DF6BD596/name "Storm"
$ETCDCTL set /servicebroker/openshift/catalog/6555DBC1-E6BC-4F0D-8948-E1B5DF6BD596/description "Storm为分布式实时计算框架。版本：v0.9.2"
$ETCDCTL set /servicebroker/openshift/catalog/6555DBC1-E6BC-4F0D-8948-E1B5DF6BD596/bindable true
$ETCDCTL set /servicebroker/openshift/catalog/6555DBC1-E6BC-4F0D-8948-E1B5DF6BD596/planupdatable false
$ETCDCTL set /servicebroker/openshift/catalog/6555DBC1-E6BC-4F0D-8948-E1B5DF6BD596/tags 'storm,openshift'
$ETCDCTL set /servicebroker/openshift/catalog/6555DBC1-E6BC-4F0D-8948-E1B5DF6BD596/metadata '{"displayName":"Storm","imageUrl":"pub/assets/Storm.png","longDescription":"Managed, highly available storm clusters in the cloud.","providerDisplayName":"Asiainfo","documentationUrl":"https://storm.apache.org/releases/1.0.1/index.html","supportUrl":"https://storm.apache.org/"}'

###创建套餐目录
$ETCDCTL mkdir /servicebroker/openshift/catalog/6555DBC1-E6BC-4F0D-8948-E1B5DF6BD596/plan
###创建套餐1
$ETCDCTL mkdir /servicebroker/openshift/catalog/6555DBC1-E6BC-4F0D-8948-E1B5DF6BD596/plan/D0B82741-770A-463C-818F-6E99862367DF
$ETCDCTL set /servicebroker/openshift/catalog/6555DBC1-E6BC-4F0D-8948-E1B5DF6BD596/plan/D0B82741-770A-463C-818F-6E99862367DF/name "standalone"
$ETCDCTL set /servicebroker/openshift/catalog/6555DBC1-E6BC-4F0D-8948-E1B5DF6BD596/plan/D0B82741-770A-463C-818F-6E99862367DF/description "单独Storm实例"
$ETCDCTL set /servicebroker/openshift/catalog/6555DBC1-E6BC-4F0D-8948-E1B5DF6BD596/plan/D0B82741-770A-463C-818F-6E99862367DF/metadata '{"bullets":["1 GB of Disk","20 connections"],"displayName":"Shared and Free" }'
$ETCDCTL set /servicebroker/openshift/catalog/6555DBC1-E6BC-4F0D-8948-E1B5DF6BD596/plan/D0B82741-770A-463C-818F-6E99862367DF/free true
###创建套餐2 external_standalone
$ETCDCTL mkdir /servicebroker/openshift/catalog/6555DBC1-E6BC-4F0D-8948-E1B5DF6BD596/plan/ef12ed9a-87f5-11e7-b949-0fc03853e5ec
$ETCDCTL set /servicebroker/openshift/catalog/6555DBC1-E6BC-4F0D-8948-E1B5DF6BD596/plan/ef12ed9a-87f5-11e7-b949-0fc03853e5ec/name "external_standalone"
$ETCDCTL set /servicebroker/openshift/catalog/6555DBC1-E6BC-4F0D-8948-E1B5DF6BD596/plan/ef12ed9a-87f5-11e7-b949-0fc03853e5ec/description "HA Storm on Openshift exposed to external"
$ETCDCTL set /servicebroker/openshift/catalog/6555DBC1-E6BC-4F0D-8948-E1B5DF6BD596/plan/ef12ed9a-87f5-11e7-b949-0fc03853e5ec/metadata '{"bullets":["1 GB of Disk","20 connections"],"displayName":"Shared and Free","customize":{"supervisors":{"default":2,"max":10,"price":100000000,"unit":"个","step":1,"desc":"Storm集群supervisor数量"},"memory":{"default":0.5,"max":32,"price":10000000,"unit":"GB","step":0.1,"desc":"Storm集群supervisor节点内存设置"},"workers":{"default":4,"max":30,"price":10000000,"unit":"个","step":1,"desc":"每个supervisor上的worker数量"}} }'
$ETCDCTL set /servicebroker/openshift/catalog/6555DBC1-E6BC-4F0D-8948-E1B5DF6BD596/plan/ef12ed9a-87f5-11e7-b949-0fc03853e5ec/free true


###创建服务 NiFi
$ETCDCTL mkdir /servicebroker/openshift/catalog/BCC10E77-98B6-4EF0-8AFB-E5FD789F712E #服务id

###创建服务级的配置
$ETCDCTL set /servicebroker/openshift/catalog/BCC10E77-98B6-4EF0-8AFB-E5FD789F712E/name "NiFi"
$ETCDCTL set /servicebroker/openshift/catalog/BCC10E77-98B6-4EF0-8AFB-E5FD789F712E/description "NiFi 是一个易于使用、功能强大而且可靠的数据处理和分发系统。版本：v0.6.1"
$ETCDCTL set /servicebroker/openshift/catalog/BCC10E77-98B6-4EF0-8AFB-E5FD789F712E/bindable true
$ETCDCTL set /servicebroker/openshift/catalog/BCC10E77-98B6-4EF0-8AFB-E5FD789F712E/planupdatable false
$ETCDCTL set /servicebroker/openshift/catalog/BCC10E77-98B6-4EF0-8AFB-E5FD789F712E/tags 'nifi,openshift'
$ETCDCTL set /servicebroker/openshift/catalog/BCC10E77-98B6-4EF0-8AFB-E5FD789F712E/metadata '{"displayName":"NiFi","imageUrl":"pub/assets/NiFiDrop.png","longDescription":"Managed, highly available nifi clusters in the cloud.","providerDisplayName":"Asiainfo","documentationUrl":"https://nifi.apache.org/docs.html","supportUrl":"https://nifi.apache.org"}'

###创建套餐目录
$ETCDCTL mkdir /servicebroker/openshift/catalog/BCC10E77-98B6-4EF0-8AFB-E5FD789F712E/plan
###创建套餐1
$ETCDCTL mkdir /servicebroker/openshift/catalog/BCC10E77-98B6-4EF0-8AFB-E5FD789F712E/plan/4435A93C-6CC9-45F0-AFB0-EA070361DD6A
$ETCDCTL set /servicebroker/openshift/catalog/BCC10E77-98B6-4EF0-8AFB-E5FD789F712E/plan/4435A93C-6CC9-45F0-AFB0-EA070361DD6A/name "standalone"
$ETCDCTL set /servicebroker/openshift/catalog/BCC10E77-98B6-4EF0-8AFB-E5FD789F712E/plan/4435A93C-6CC9-45F0-AFB0-EA070361DD6A/description "单独nifi实例"
$ETCDCTL set /servicebroker/openshift/catalog/BCC10E77-98B6-4EF0-8AFB-E5FD789F712E/plan/4435A93C-6CC9-45F0-AFB0-EA070361DD6A/metadata '{"bullets":["1 GB of Disk","20 connections"],"displayName":"Shared and Free" }'
$ETCDCTL set /servicebroker/openshift/catalog/BCC10E77-98B6-4EF0-8AFB-E5FD789F712E/plan/4435A93C-6CC9-45F0-AFB0-EA070361DD6A/free true



###创建服务 Kettle
$ETCDCTL mkdir /servicebroker/openshift/catalog/51219A87-E37E-44F5-8E30-4767348D644D #服务id

###创建服务级的配置
$ETCDCTL set /servicebroker/openshift/catalog/51219A87-E37E-44F5-8E30-4767348D644D/name "Kettle"
$ETCDCTL set /servicebroker/openshift/catalog/51219A87-E37E-44F5-8E30-4767348D644D/description "Kettle是一款国外开源的ETL工具，数据抽取高效稳定。版本：v5.0.1"
$ETCDCTL set /servicebroker/openshift/catalog/51219A87-E37E-44F5-8E30-4767348D644D/bindable true
$ETCDCTL set /servicebroker/openshift/catalog/51219A87-E37E-44F5-8E30-4767348D644D/planupdatable false
$ETCDCTL set /servicebroker/openshift/catalog/51219A87-E37E-44F5-8E30-4767348D644D/tags 'kettle,openshift'
$ETCDCTL set /servicebroker/openshift/catalog/51219A87-E37E-44F5-8E30-4767348D644D/metadata '{"displayName":"Kettle","imageUrl":"pub/assets/Kettle.png","longDescription":"Managed, highly available kettle clusters in the cloud.","providerDisplayName":"Asiainfo","documentationUrl":"http://wiki.pentaho.com/display/EAI/Latest+Pentaho+Data+Integration+%28aka+Kettle%29+Documentation","supportUrl":"http://community.pentaho.com/projects/data-integration/"}'

###创建套餐目录
$ETCDCTL mkdir /servicebroker/openshift/catalog/51219A87-E37E-44F5-8E30-4767348D644D/plan
###创建套餐1
$ETCDCTL mkdir /servicebroker/openshift/catalog/51219A87-E37E-44F5-8E30-4767348D644D/plan/31B428F8-AA3E-4CAC-85A2-7294C7CAA79D
$ETCDCTL set /servicebroker/openshift/catalog/51219A87-E37E-44F5-8E30-4767348D644D/plan/31B428F8-AA3E-4CAC-85A2-7294C7CAA79D/name "standalone"
$ETCDCTL set /servicebroker/openshift/catalog/51219A87-E37E-44F5-8E30-4767348D644D/plan/31B428F8-AA3E-4CAC-85A2-7294C7CAA79D/description "单独kettle实例"
$ETCDCTL set /servicebroker/openshift/catalog/51219A87-E37E-44F5-8E30-4767348D644D/plan/31B428F8-AA3E-4CAC-85A2-7294C7CAA79D/metadata '{"bullets":["1 GB of Disk","20 connections"],"displayName":"Shared and Free" }'
$ETCDCTL set /servicebroker/openshift/catalog/51219A87-E37E-44F5-8E30-4767348D644D/plan/31B428F8-AA3E-4CAC-85A2-7294C7CAA79D/free true



###创建服务 TensorFlow
$ETCDCTL mkdir /servicebroker/openshift/catalog/2DD1363D-8768-4DAA-BDC3-FB29C10C4A8C #服务id

###创建服务级的配置
$ETCDCTL set /servicebroker/openshift/catalog/2DD1363D-8768-4DAA-BDC3-FB29C10C4A8C/name "TensorFlow"
$ETCDCTL set /servicebroker/openshift/catalog/2DD1363D-8768-4DAA-BDC3-FB29C10C4A8C/description "谷歌基于DistBelief研发的第二代AI学习系统。版本：v0.8.0"
$ETCDCTL set /servicebroker/openshift/catalog/2DD1363D-8768-4DAA-BDC3-FB29C10C4A8C/bindable true
$ETCDCTL set /servicebroker/openshift/catalog/2DD1363D-8768-4DAA-BDC3-FB29C10C4A8C/planupdatable false
$ETCDCTL set /servicebroker/openshift/catalog/2DD1363D-8768-4DAA-BDC3-FB29C10C4A8C/tags 'tensorflow,openshift'
$ETCDCTL set /servicebroker/openshift/catalog/2DD1363D-8768-4DAA-BDC3-FB29C10C4A8C/metadata '{"displayName":"TensorFlow","imageUrl":"pub/assets/TensorFlow.png","longDescription":"Managed, highly available tensorflow clusters in the cloud.","providerDisplayName":"Asiainfo","documentationUrl":"https://www.tensorflow.org/get_started","supportUrl":"https://www.tensorflow.org/"}'

###创建套餐目录
$ETCDCTL mkdir /servicebroker/openshift/catalog/2DD1363D-8768-4DAA-BDC3-FB29C10C4A8C/plan
###创建套餐1
$ETCDCTL mkdir /servicebroker/openshift/catalog/2DD1363D-8768-4DAA-BDC3-FB29C10C4A8C/plan/BE1CAAF2-CAB7-4B56-AEB4-2A3111225D50
$ETCDCTL set /servicebroker/openshift/catalog/2DD1363D-8768-4DAA-BDC3-FB29C10C4A8C/plan/BE1CAAF2-CAB7-4B56-AEB4-2A3111225D50/name "standalone"
$ETCDCTL set /servicebroker/openshift/catalog/2DD1363D-8768-4DAA-BDC3-FB29C10C4A8C/plan/BE1CAAF2-CAB7-4B56-AEB4-2A3111225D50/description "单独tensorflow实例"
$ETCDCTL set /servicebroker/openshift/catalog/2DD1363D-8768-4DAA-BDC3-FB29C10C4A8C/plan/BE1CAAF2-CAB7-4B56-AEB4-2A3111225D50/metadata '{"bullets":["1 GB of Disk","20 connections"],"displayName":"Shared and Free" }'
$ETCDCTL set /servicebroker/openshift/catalog/2DD1363D-8768-4DAA-BDC3-FB29C10C4A8C/plan/BE1CAAF2-CAB7-4B56-AEB4-2A3111225D50/free true




###创建服务 PySpider
$ETCDCTL mkdir /servicebroker/openshift/catalog/c6ed3cb8-d486-4faa-8185-7262183a1113 #服务id

###创建服务级的配置
$ETCDCTL set /servicebroker/openshift/catalog/c6ed3cb8-d486-4faa-8185-7262183a1113/name "PySpider"
$ETCDCTL set /servicebroker/openshift/catalog/c6ed3cb8-d486-4faa-8185-7262183a1113/description "A Sample PySpider (v0.3.7) cluster on Openshift"
$ETCDCTL set /servicebroker/openshift/catalog/c6ed3cb8-d486-4faa-8185-7262183a1113/bindable true
$ETCDCTL set /servicebroker/openshift/catalog/c6ed3cb8-d486-4faa-8185-7262183a1113/planupdatable false
$ETCDCTL set /servicebroker/openshift/catalog/c6ed3cb8-d486-4faa-8185-7262183a1113/tags 'pyspider,openshift'
$ETCDCTL set /servicebroker/openshift/catalog/c6ed3cb8-d486-4faa-8185-7262183a1113/metadata '{"displayName":"PySpider","imageUrl":"","longDescription":"A Powerful Spider(Web Crawler) System in Python.","providerDisplayName":"Asiainfo","documentationUrl":"https://docs.pyspider.org","supportUrl":"https://github.com/binux/pyspider"}'

###创建套餐目录
$ETCDCTL mkdir /servicebroker/openshift/catalog/c6ed3cb8-d486-4faa-8185-7262183a1113/plan
###创建套餐1
$ETCDCTL mkdir /servicebroker/openshift/catalog/c6ed3cb8-d486-4faa-8185-7262183a1113/plan/1f195802-1642-47e9-be69-9082cc1ceaf5
$ETCDCTL set /servicebroker/openshift/catalog/c6ed3cb8-d486-4faa-8185-7262183a1113/plan/1f195802-1642-47e9-be69-9082cc1ceaf5/name "standalone"
$ETCDCTL set /servicebroker/openshift/catalog/c6ed3cb8-d486-4faa-8185-7262183a1113/plan/1f195802-1642-47e9-be69-9082cc1ceaf5/description "HA Spider on Openshift"
$ETCDCTL set /servicebroker/openshift/catalog/c6ed3cb8-d486-4faa-8185-7262183a1113/plan/1f195802-1642-47e9-be69-9082cc1ceaf5/metadata '{"bullets":["1 GB of Disk","20 connections"],"displayName":"Shared and Free" }'
$ETCDCTL set /servicebroker/openshift/catalog/c6ed3cb8-d486-4faa-8185-7262183a1113/plan/1f195802-1642-47e9-be69-9082cc1ceaf5/free true




###创建服务 MongoDB
$ETCDCTL mkdir /servicebroker/openshift/catalog/eac6c8cf-2a63-4120-9e29-30c4b716e37f #服务id

###创建服务级的配置
$ETCDCTL set /servicebroker/openshift/catalog/eac6c8cf-2a63-4120-9e29-30c4b716e37f/name "Mongo"
$ETCDCTL set /servicebroker/openshift/catalog/eac6c8cf-2a63-4120-9e29-30c4b716e37f/description "A Sample MongoDB cluster on Openshift"
$ETCDCTL set /servicebroker/openshift/catalog/eac6c8cf-2a63-4120-9e29-30c4b716e37f/bindable true
$ETCDCTL set /servicebroker/openshift/catalog/eac6c8cf-2a63-4120-9e29-30c4b716e37f/planupdatable false
$ETCDCTL set /servicebroker/openshift/catalog/eac6c8cf-2a63-4120-9e29-30c4b716e37f/tags 'mongodb,openshift'
$ETCDCTL set /servicebroker/openshift/catalog/eac6c8cf-2a63-4120-9e29-30c4b716e37f/metadata '{"displayName":"MongoDB","imageUrl":"https://webassets.mongodb.com/_com_assets/global/mongodb-logo-white.png","longDescription":"Managed, highly available MongoDB clusters in the cloud.","providerDisplayName":"Asiainfo","documentationUrl":"https://docs.mongodb.com/","supportUrl":"https://www.mongodb.com/"}'

###创建套餐目录
$ETCDCTL mkdir /servicebroker/openshift/catalog/eac6c8cf-2a63-4120-9e29-30c4b716e37f/plan
###创建套餐1 (pvc)
$ETCDCTL mkdir /servicebroker/openshift/catalog/eac6c8cf-2a63-4120-9e29-30c4b716e37f/plan/3b7fc05d-c630-4c0b-8dda-2cedb271ccb5
$ETCDCTL set /servicebroker/openshift/catalog/eac6c8cf-2a63-4120-9e29-30c4b716e37f/plan/3b7fc05d-c630-4c0b-8dda-2cedb271ccb5/name "volumes_standalone"
$ETCDCTL set /servicebroker/openshift/catalog/eac6c8cf-2a63-4120-9e29-30c4b716e37f/plan/3b7fc05d-c630-4c0b-8dda-2cedb271ccb5/description "HA MongoDB With Volumes on Openshift"
$ETCDCTL set /servicebroker/openshift/catalog/eac6c8cf-2a63-4120-9e29-30c4b716e37f/plan/3b7fc05d-c630-4c0b-8dda-2cedb271ccb5/metadata '{"bullets":["1 GB of Disk","20 connections"],"displayName":"Shared and Free" }'
$ETCDCTL set /servicebroker/openshift/catalog/eac6c8cf-2a63-4120-9e29-30c4b716e37f/plan/3b7fc05d-c630-4c0b-8dda-2cedb271ccb5/free false






###创建服务 Elasticsearch
$ETCDCTL mkdir /servicebroker/openshift/catalog/3626D834-BD32-11E6-8C01-F7A6E255AF47

$ETCDCTL set /servicebroker/openshift/catalog/3626D834-BD32-11E6-8C01-F7A6E255AF47/name "Elasticsearch"
$ETCDCTL set /servicebroker/openshift/catalog/3626D834-BD32-11E6-8C01-F7A6E255AF47/description "A Sample Elasticsearch (v2.3.0) cluster on Openshift"
$ETCDCTL set /servicebroker/openshift/catalog/3626D834-BD32-11E6-8C01-F7A6E255AF47/bindable true
$ETCDCTL set /servicebroker/openshift/catalog/3626D834-BD32-11E6-8C01-F7A6E255AF47/planupdatable false
$ETCDCTL set /servicebroker/openshift/catalog/3626D834-BD32-11E6-8C01-F7A6E255AF47/tags 'Elasticsearch,openshift'
$ETCDCTL set /servicebroker/openshift/catalog/3626D834-BD32-11E6-8C01-F7A6E255AF47/metadata '{"displayName":"Elasticsearch","imageUrl":"pub/assets/ElasticSearch.png","longDescription":"Managed, highly available etcd clusters in the cloud.","providerDisplayName":"Asiainfo","documentationUrl":"https://coreos.com/etcd/docs/latest","supportUrl":"https://coreos.com"}'


###创建套餐目录
$ETCDCTL mkdir /servicebroker/openshift/catalog/3626D834-BD32-11E6-8C01-F7A6E255AF47/plan
###创建套餐(pvc)
$ETCDCTL mkdir /servicebroker/openshift/catalog/3626D834-BD32-11E6-8C01-F7A6E255AF47/plan/A9537ABE-BD33-11E6-A969-13A2D25B7667
$ETCDCTL set /servicebroker/openshift/catalog/3626D834-BD32-11E6-8C01-F7A6E255AF47/plan/A9537ABE-BD33-11E6-A969-13A2D25B7667/name "volumes_standalone"
$ETCDCTL set /servicebroker/openshift/catalog/3626D834-BD32-11E6-8C01-F7A6E255AF47/plan/A9537ABE-BD33-11E6-A969-13A2D25B7667/description "HA Elasticsearch on Openshift"
$ETCDCTL set /servicebroker/openshift/catalog/3626D834-BD32-11E6-8C01-F7A6E255AF47/plan/A9537ABE-BD33-11E6-A969-13A2D25B7667/metadata '{"bullets":["20 GB of Disk","20 connections"],"displayName":"Shared and Free" }'
$ETCDCTL set /servicebroker/openshift/catalog/3626D834-BD32-11E6-8C01-F7A6E255AF47/plan/A9537ABE-BD33-11E6-A969-13A2D25B7667/free true

###创建套餐(cluster)
$ETCDCTL mkdir /servicebroker/openshift/catalog/3626D834-BD32-11E6-8C01-F7A6E255AF47/plan/95cd832a-52a3-11e8-92bf-fa163e3e1b52
$ETCDCTL set /servicebroker/openshift/catalog/3626D834-BD32-11E6-8C01-F7A6E255AF47/plan/95cd832a-52a3-11e8-92bf-fa163e3e1b52/name "Cluster"
$ETCDCTL set /servicebroker/openshift/catalog/3626D834-BD32-11E6-8C01-F7A6E255AF47/plan/95cd832a-52a3-11e8-92bf-fa163e3e1b52/description "Elasticsearch (6.2.2) Cluster on Openshift"
$ETCDCTL set /servicebroker/openshift/catalog/3626D834-BD32-11E6-8C01-F7A6E255AF47/plan/95cd832a-52a3-11e8-92bf-fa163e3e1b52/metadata '{"bullets":["3 replicas", "20 GB of Disk", "0.5 cpu", "2G memory"],"displayName":"Shared and Free", "customize":{"replicas":{"default":3,"unit":"nodes","step":1, "max": 30, "desc":"Elasticsearch集群实例数量"},"volume":{"default":5,"unit":"GB","step":1, "max":100, "desc":"单个实例存储容量设置"}, "cpu":{"default":0.5,"unit":"core","step":0.1, "max": 2, "desc":"单个实例分配的cpu数量"}, "mem":{"default":2,"unit":"Gi","step":1, "max": 24, "desc":"单个实例分配的内存数量"}} }'
$ETCDCTL set /servicebroker/openshift/catalog/3626D834-BD32-11E6-8C01-F7A6E255AF47/plan/95cd832a-52a3-11e8-92bf-fa163e3e1b52/free true





###创建服务 Dataiku
$ETCDCTL mkdir /servicebroker/openshift/catalog/e4871703-a37e-427b-bbfc-313c1fbf685f

$ETCDCTL set /servicebroker/openshift/catalog/e4871703-a37e-427b-bbfc-313c1fbf685f/name "Dataiku"
$ETCDCTL set /servicebroker/openshift/catalog/e4871703-a37e-427b-bbfc-313c1fbf685f/description "Dataiku on Openshift"
$ETCDCTL set /servicebroker/openshift/catalog/e4871703-a37e-427b-bbfc-313c1fbf685f/bindable true
$ETCDCTL set /servicebroker/openshift/catalog/e4871703-a37e-427b-bbfc-313c1fbf685f/planupdatable false
$ETCDCTL set /servicebroker/openshift/catalog/e4871703-a37e-427b-bbfc-313c1fbf685f/tags 'Dataiku,openshift'
$ETCDCTL set /servicebroker/openshift/catalog/e4871703-a37e-427b-bbfc-313c1fbf685f/metadata '{"displayName":"Dataiku","imageUrl":"pub/assets/Dataiku.png","longDescription":"Dataiku in the cloud.","providerDisplayName":"Asiainfo","documentationUrl":"https://www.dataiku.com/learn/","supportUrl":"https://www.dataiku.com"}'

###创建套餐目录
$ETCDCTL mkdir /servicebroker/openshift/catalog/e4871703-a37e-427b-bbfc-313c1fbf685f/plan
###创建套餐1(dataiku_standalone)
#$ETCDCTL mkdir /servicebroker/openshift/catalog/e4871703-a37e-427b-bbfc-313c1fbf685f/plan/3286b8bb-790b-40bc-aeaf-46a0a788f1cc
#$ETCDCTL set /servicebroker/openshift/catalog/e4871703-a37e-427b-bbfc-313c1fbf685f/plan/3286b8bb-790b-40bc-aeaf-46a0a788f1cc/name "standalone"
#$ETCDCTL set /servicebroker/openshift/catalog/e4871703-a37e-427b-bbfc-313c1fbf685f/plan/3286b8bb-790b-40bc-aeaf-46a0a788f1cc/description "Dataiku on Openshift"
#$ETCDCTL set /servicebroker/openshift/catalog/e4871703-a37e-427b-bbfc-313c1fbf685f/plan/3286b8bb-790b-40bc-aeaf-46a0a788f1cc/metadata '{"bullets":["1 GB of Disk","20 connections"],"displayName":"Shared and Free","customize":{"memory":{"default":2,"max":32,"price":10000000,"unit":"GB","step":0.1,"desc":"Dataiku节点内存设置"},"cpu":{"default":1,"max":16,"price":10000000,"unit":"个","step":0.1,"desc":"Dataiku的cpu数量"}} }'
#$ETCDCTL set /servicebroker/openshift/catalog/e4871703-a37e-427b-bbfc-313c1fbf685f/plan/3286b8bb-790b-40bc-aeaf-46a0a788f1cc/free true

###创建套餐2(dataiku_pvc_standalone)
$ETCDCTL mkdir /servicebroker/openshift/catalog/e4871703-a37e-427b-bbfc-313c1fbf685f/plan/133F32F2-6E8A-4CD6-A14C-1D400866B6B3
$ETCDCTL set /servicebroker/openshift/catalog/e4871703-a37e-427b-bbfc-313c1fbf685f/plan/133F32F2-6E8A-4CD6-A14C-1D400866B6B3/name "volumes_standalone"
$ETCDCTL set /servicebroker/openshift/catalog/e4871703-a37e-427b-bbfc-313c1fbf685f/plan/133F32F2-6E8A-4CD6-A14C-1D400866B6B3/description "HA Dataiku on Openshift"
$ETCDCTL set /servicebroker/openshift/catalog/e4871703-a37e-427b-bbfc-313c1fbf685f/plan/133F32F2-6E8A-4CD6-A14C-1D400866B6B3/metadata '{"bullets":["1 GB of Disk","20 connections"],"displayName":"Shared and Free","customize":{"memory":{"default":2,"max":32,"price":10000000,"unit":"GB","step":0.1,"desc":"Dataiku节点内存设置"},"cpu":{"default":1,"max":16,"price":10000000,"unit":"个","step":0.1,"desc":"Dataiku的cpu数量"}} }'
$ETCDCTL set /servicebroker/openshift/catalog/e4871703-a37e-427b-bbfc-313c1fbf685f/plan/133F32F2-6E8A-4CD6-A14C-1D400866B6B3/free true




###创建服务 Zeppelin
$ETCDCTL set /servicebroker/openshift/catalog/A326EF4F-74D0-4B60-9CA0-CAED94D7E50F/name "Zeppelin"
$ETCDCTL set /servicebroker/openshift/catalog/A326EF4F-74D0-4B60-9CA0-CAED94D7E50F/description "Zeppelin on Openshift"
$ETCDCTL set /servicebroker/openshift/catalog/A326EF4F-74D0-4B60-9CA0-CAED94D7E50F/bindable true
$ETCDCTL set /servicebroker/openshift/catalog/A326EF4F-74D0-4B60-9CA0-CAED94D7E50F/planupdatable false
$ETCDCTL set /servicebroker/openshift/catalog/A326EF4F-74D0-4B60-9CA0-CAED94D7E50F/tags 'Zeppelin,openshift'
$ETCDCTL set /servicebroker/openshift/catalog/A326EF4F-74D0-4B60-9CA0-CAED94D7E50F/metadata '{"displayName":"Zeppelin","imageUrl":"pub/assets/Zeppelin.png","longDescription":"Zeppelin in the cloud.","providerDisplayName":"Asiainfo","documentationUrl":"http://zeppelin.apache.org/docs/0.7.3/","supportUrl":"http://zeppelin.apache.org"}'

###创建套餐目录
$ETCDCTL mkdir /servicebroker/openshift/catalog/A326EF4F-74D0-4B60-9CA0-CAED94D7E50F/plan
###创建套餐(standalone)
$ETCDCTL mkdir /servicebroker/openshift/catalog/A326EF4F-74D0-4B60-9CA0-CAED94D7E50F/plan/F344A7E1-2E49-4BA4-8BEF-42FFCC7AEB14
$ETCDCTL set /servicebroker/openshift/catalog/A326EF4F-74D0-4B60-9CA0-CAED94D7E50F/plan/F344A7E1-2E49-4BA4-8BEF-42FFCC7AEB14/name "standalone"
$ETCDCTL set /servicebroker/openshift/catalog/A326EF4F-74D0-4B60-9CA0-CAED94D7E50F/plan/F344A7E1-2E49-4BA4-8BEF-42FFCC7AEB14/description "Zeppelin on Openshift"
$ETCDCTL set /servicebroker/openshift/catalog/A326EF4F-74D0-4B60-9CA0-CAED94D7E50F/plan/F344A7E1-2E49-4BA4-8BEF-42FFCC7AEB14/metadata '{"bullets":["20 GB of Disk","20 connections"],"displayName":"Shared and Free","customize":{"memory":{"default":2,"max":32,"price":10000000,"unit":"GB","step":0.1,"desc":"Zeppelin节点内存设置"},"cpu":{"default":1,"max":16,"price":10000000,"unit":"个","step":0.1,"desc":"Zeppelin的cpu数量"}} }'
$ETCDCTL set /servicebroker/openshift/catalog/A326EF4F-74D0-4B60-9CA0-CAED94D7E50F/plan/F344A7E1-2E49-4BA4-8BEF-42FFCC7AEB14/free true




###创建服务 Anaconda
$ETCDCTL mkdir /servicebroker/openshift/catalog/dfc126e9-181a-4d13-a367-f84edfe617ed

$ETCDCTL set /servicebroker/openshift/catalog/dfc126e9-181a-4d13-a367-f84edfe617ed/name "Anaconda"
$ETCDCTL set /servicebroker/openshift/catalog/dfc126e9-181a-4d13-a367-f84edfe617ed/description "Anaconda on Openshift"
$ETCDCTL set /servicebroker/openshift/catalog/dfc126e9-181a-4d13-a367-f84edfe617ed/bindable true
$ETCDCTL set /servicebroker/openshift/catalog/dfc126e9-181a-4d13-a367-f84edfe617ed/planupdatable false
$ETCDCTL set /servicebroker/openshift/catalog/dfc126e9-181a-4d13-a367-f84edfe617ed/tags 'Anaconda,openshift'
$ETCDCTL set /servicebroker/openshift/catalog/dfc126e9-181a-4d13-a367-f84edfe617ed/metadata '{"displayName":"Anaconda","imageUrl":"pub/assets/Anaconda.png","longDescription":"Anaconda in the cloud.","providerDisplayName":"Asiainfo","documentationUrl":"https://docs.anaconda.com/","supportUrl":"https://www.anaconda.com/support/"}'

###创建套餐目录
$ETCDCTL mkdir /servicebroker/openshift/catalog/dfc126e9-181a-4d13-a367-f84edfe617ed/plan
###创建套餐(pvc)
$ETCDCTL mkdir /servicebroker/openshift/catalog/dfc126e9-181a-4d13-a367-f84edfe617ed/plan/521a4a06-175a-43e6-b1bc-d9c684f76a0d
$ETCDCTL set /servicebroker/openshift/catalog/dfc126e9-181a-4d13-a367-f84edfe617ed/plan/521a4a06-175a-43e6-b1bc-d9c684f76a0d/name "standalone"
$ETCDCTL set /servicebroker/openshift/catalog/dfc126e9-181a-4d13-a367-f84edfe617ed/plan/521a4a06-175a-43e6-b1bc-d9c684f76a0d/description "Anaconda on Openshift"
$ETCDCTL set /servicebroker/openshift/catalog/dfc126e9-181a-4d13-a367-f84edfe617ed/plan/521a4a06-175a-43e6-b1bc-d9c684f76a0d/metadata '{"bullets":["1 GB of Disk","20 connections"],"displayName":"Shared and Free" ,"customize":{"memory":{"default":2,"max":32,"price":10000000,"unit":"GB","step":0.1,"desc":"Anaconda节点内存设置"},"cpu":{"default":1,"max":16,"price":10000000,"unit":"个","step":0.1,"desc":"Anaconda的cpu数量"}} }'
$ETCDCTL set /servicebroker/openshift/catalog/dfc126e9-181a-4d13-a367-f84edfe617ed/plan/521a4a06-175a-43e6-b1bc-d9c684f76a0d/free true
###创建套餐(volume_pvc)
$ETCDCTL mkdir /servicebroker/openshift/catalog/dfc126e9-181a-4d13-a367-f84edfe617ed/plan/a321538f-dee1-4511-93fd-96ade8fee82e
$ETCDCTL set /servicebroker/openshift/catalog/dfc126e9-181a-4d13-a367-f84edfe617ed/plan/a321538f-dee1-4511-93fd-96ade8fee82e/name "pvc_standalone"
$ETCDCTL set /servicebroker/openshift/catalog/dfc126e9-181a-4d13-a367-f84edfe617ed/plan/a321538f-dee1-4511-93fd-96ade8fee82e/description "Anaconda on Openshift"
$ETCDCTL set /servicebroker/openshift/catalog/dfc126e9-181a-4d13-a367-f84edfe617ed/plan/a321538f-dee1-4511-93fd-96ade8fee82e/metadata '{"bullets":["1 GB of Disk","20 connections"],"displayName":"Shared and Free" ,"customize":{"memory":{"default":2,"max":32,"price":10000000,"unit":"GB","step":0.1,"desc":"Anaconda节点内存设置"},"cpu":{"default":1,"max":16,"price":10000000,"unit":"个","step":0.1,"desc":"Anaconda的cpu数量"}} }'
$ETCDCTL set /servicebroker/openshift/catalog/dfc126e9-181a-4d13-a367-f84edfe617ed/plan/a321538f-dee1-4511-93fd-96ade8fee82e/free true





###创建服务 MySQL
$ETCDCTL mkdir /servicebroker/openshift/catalog/0f96b0f0-6a25-4018-8225-8f1cd090b1f9 #服务id

###创建服务级的配置
$ETCDCTL set /servicebroker/openshift/catalog/0f96b0f0-6a25-4018-8225-8f1cd090b1f9/name "MySQL"
$ETCDCTL set /servicebroker/openshift/catalog/0f96b0f0-6a25-4018-8225-8f1cd090b1f9/description "A Sample MySQL cluster on Openshift"
$ETCDCTL set /servicebroker/openshift/catalog/0f96b0f0-6a25-4018-8225-8f1cd090b1f9/bindable true
$ETCDCTL set /servicebroker/openshift/catalog/0f96b0f0-6a25-4018-8225-8f1cd090b1f9/planupdatable false
$ETCDCTL set /servicebroker/openshift/catalog/0f96b0f0-6a25-4018-8225-8f1cd090b1f9/tags 'mysql,openshift'
$ETCDCTL set /servicebroker/openshift/catalog/0f96b0f0-6a25-4018-8225-8f1cd090b1f9/metadata '{"displayName":"MySQL","imageUrl":"https://labs.mysql.com/common/logos/mysql-logo.svg?v2","longDescription":"Managed, highly available MySQL clusters in the cloud.","providerDisplayName":"Asiainfo","documentationUrl":"https://dev.mysql.com/doc/","supportUrl":"https://www.mysql.com/"}'

###创建套餐目录
$ETCDCTL mkdir /servicebroker/openshift/catalog/0f96b0f0-6a25-4018-8225-8f1cd090b1f9/plan
###创建套餐1 (pvc)
$ETCDCTL mkdir /servicebroker/openshift/catalog/0f96b0f0-6a25-4018-8225-8f1cd090b1f9/plan/5a648266-4a76-4ab3-98ab-fe8933de161a
$ETCDCTL set /servicebroker/openshift/catalog/0f96b0f0-6a25-4018-8225-8f1cd090b1f9/plan/5a648266-4a76-4ab3-98ab-fe8933de161a/name "volumes_ha_cluster"
$ETCDCTL set /servicebroker/openshift/catalog/0f96b0f0-6a25-4018-8225-8f1cd090b1f9/plan/5a648266-4a76-4ab3-98ab-fe8933de161a/description "HA MySQL With Volumes on Openshift"
$ETCDCTL set /servicebroker/openshift/catalog/0f96b0f0-6a25-4018-8225-8f1cd090b1f9/plan/5a648266-4a76-4ab3-98ab-fe8933de161a/metadata '{"bullets":["1 GB of Disk","20 connections"],"displayName":"Shared and Free" }'
$ETCDCTL set /servicebroker/openshift/catalog/0f96b0f0-6a25-4018-8225-8f1cd090b1f9/plan/5a648266-4a76-4ab3-98ab-fe8933de161a/free false




