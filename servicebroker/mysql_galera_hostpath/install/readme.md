

# 安装 operator_mariadb_galera

### 上传镜像

将镜像上传放置到合适位置（须保证k8s集群可以拉取）：
1. mariadb:10.2-custom-v21 （此镜像地址将配置在operator_mariadb_galera程序环境变量中）
1. phpmyadmin:4.6 （此镜像地址将配置在operator_mariadb_galera程序环境变量中）
1. operator_mariadb_galera:latest (当前项目镜像)
1. etcd:2.3.1 (注册表使用)

### (k8s集群管理员) 在k8s集群中创建一个namespace, operator_mariadb_galera程序资源（deployment）将创建在此namespace中。

推荐namespace名: asiainfo-operators

### (k8s集群管理员) 在k8s集群中创建一个namespace, 所有的garela集群的资源（svc, statefulset等）将创建存储在此namespace中。

为方便，此namespace可以和上面的namespace使用同一个。

以下假设此namespace为 mariadb-galera-instances

此namespace名称和一个对此namespace有创建将配置在operator_mariadb_galera程序环境变量中.

### (k8s集群管理员) 在k8s集群中存储garela资源的namespace下创建一个特殊的ServiceAccount

使用此ServiceAccount的pods将有权访问node本地卷（hostPath）。假设此ServiceAccount为 hostpathuser

```
kubectl create serviceaccount -n mariadb-galera-instances hostpathuser
kubectl adm policy add-scc-to-user privileged -n mariadb-galera-instances -z hostpathuser
```

此ServiceAccount将配置在operator_mariadb_galera程序环境变量中.

### (k8s集群管理员) 在k8s集群中运行MySQL pods的三个nodes上创建MySQL数据目录

Example:
```
# log into node 0/1/2
mkdir -p /asiainfo-operators/mariadb-galera
chmod 777 /asiainfo-operators/mariadb-galera
```

每个Galera集群的数据将会存储在此目录下的一个子目录。
* 其中operator_mariadb_galera程序的数据子目录名为operator-data
* 其它每个galera实例对应的子目录名为galera-xxx (xxx为instance Id)
```

### (k8s集群管理员) 给k8s集群中运行MySQL pods的三个nodes打label

所有MariaDB pod将只会起在这三个node上。

Example:
```
kubectl label nodes node-name-0 asiainfo-operator-mariadb-galera=host-path
kubectl label nodes node-name-1 asiainfo-operator-mariadb-galera=host-path
kubectl label nodes node-name-2 asiainfo-operator-mariadb-galera=host-path
```

### 启动 etcd 程序

### 写入 etcd 注册表

```

ETCD_USER=xxx
ETCD_PASSWORD=xxx
ETCD_ADDRESS=http://xxx:2379

export ETCDCTL="etcdctl --timeout 15s --total-timeout 30s --endpoints $ETCD_ADDRESS --username $ETCD_USER:$ETCD_PASSWORD"

API_NAME=xxx
API_PASSWORD=xxx

$ETCDCTL mkdir /servicebroker
$ETCDCTL mkdir /servicebroker/openshift
$ETCDCTL set /servicebroker/openshift/username $API_NAME
$ETCDCTL set /servicebroker/openshift/password $API_PASSWORD

$ETCDCTL mkdir /servicebroker/openshift/instance

$ETCDCTL mkdir /servicebroker/openshift/catalog


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

###创建套餐2 (hostpath)
$ETCDCTL mkdir /servicebroker/openshift/catalog/0f96b0f0-6a25-4018-8225-8f1cd090b1f9/plan/b80b0b7d-5108-4038-b560-67d82e6a43b7
$ETCDCTL set /servicebroker/openshift/catalog/0f96b0f0-6a25-4018-8225-8f1cd090b1f9/plan/b80b0b7d-5108-4038-b560-67d82e6a43b7/name "hostpath_ha_cluster"
$ETCDCTL set /servicebroker/openshift/catalog/0f96b0f0-6a25-4018-8225-8f1cd090b1f9/plan/b80b0b7d-5108-4038-b560-67d82e6a43b7/description "HA MySQL With HostPath Support on Kubernetes"
$ETCDCTL set /servicebroker/openshift/catalog/0f96b0f0-6a25-4018-8225-8f1cd090b1f9/plan/b80b0b7d-5108-4038-b560-67d82e6a43b7/metadata '{"bullets":["1 GB of Disk","20 connections"],"displayName":"Shared and Free" }'
$ETCDCTL set /servicebroker/openshift/catalog/0f96b0f0-6a25-4018-8225-8f1cd090b1f9/plan/b80b0b7d-5108-4038-b560-67d82e6a43b7/free false
```

### 启动 operator_mariadb_galera 程序

修改下面yaml中的
* operator_mariadb_galera image
* hostPath volume: 
* serice account name
* enviroment variables
  * mariadb image
  * phpmysqladmin image
  * hostPath volume
  * service instance namespace
  * privileged account service

注意：需要的环境变量
```
        - name: NODE_ADDRESSES
          value: 10.1.234.35,10.1.234.36,10.1.234.37,10.1.234.38


        - name: HOSTPATHSERVICEACCOUNT
          value: hostpathuser
        - name: MARIADBGALERAHOSTPATHDATAPATH
          value: /test/mariadb
        - name: MARIADBGALERAHOSTPATHNODELABELS
          value: qaz=741
```


(k8s集群管理员) 运行:
```
kubectl create -f operator_mariadb_galera.yaml
```

# 通过 operator_mariadb_galera 程序创建新的 mariadb galera 集群服务实例

```

# provision
curl -i -X PUT http://xxx/v2/service_instances/mysql-host-path-test -d '{
  "service_id":"0f96b0f0-6a25-4018-8225-8f1cd090b1f9",
  "plan_id":"b80b0b7d-5108-4038-b560-67d82e6a43b7",
  "organization_guid": "default",
  "space_guid":"space-guid",
  "accepts_incomplete":true,
  "parameters": {"ami_id":"ami-ecb68a84"}
}' -H "Content-Type: application/json"


# deprovision
curl -i -X DELETE -L 'http://xxx/v2/service_instances/mysql-host-path-test?service_id=0f96b0f0-6a25-4018-8225-8f1cd090b1f9&plan_id=b80b0b7d-5108-4038-b560-67d82e6a43b7'

```




