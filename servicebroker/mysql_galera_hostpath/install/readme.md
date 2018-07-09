

Need:
* 安装文档。
* MySQL config修改并重启MySQL集群。
* pod/contianer CPU和memory配额限制。
* 带图表的独立监控系统。
  * 创建一个独立的Promethous + MySQL Exporter。
  * 或者使用D3.js + custom data collector。
  * ref
    * https://hub.docker.com/r/logzio/mysql-monitor/
    * https://github.com/bmildren/docker-mysql-monitoring
    * galera official monitor
    * 天兔
* 限制表空间大小。
  * MySQL本身并不支持此功能。
  * 一般利用磁盘的物理极限来做此限制。
  * 或者可以监视MySQL的数据使用量 + 限制INSERT/UPDATE权限的方式。（很难做得靠谱, ref: http://projects.marsching.org/mysql_quota/）
  * 或者使用kubernetes 1.10中提供的Local Persistent Volumes。
* 网络隔离。
  * 如果使用Calico，可以将每个MySQL集群开在不同的namespace下。（需要service-broker程序使用的ServiceAccount既有更多的权限）
  * 或者可以不用service-broker程序，直接提供几个yaml模板参考，直接使用UI创建MySQL集群的Deployment/Service/...
* StatefulSet -> Deployment


# 当前galera的限制

目前使用MariaDB版本是10.2.15.

https://mariadb.com/kb/en/library/mariadb-galera-cluster-known-limitations/
* All tables should have a primary key.
* https://mariadb.com/kb/en/library/galera-cluster-system-variables/#wsrep_max_ws_rows: 可以设成0，表示没限制.
* https://mariadb.com/kb/en/library/galera-cluster-system-variables/#wsrep_max_ws_size: 最大2GB

# 安装 service_broker_mariadb_galera

### (k8s集群管理员) 上传镜像

将两个镜像上传放置到合适位置（须保证k8s集群可以拉取）：
1. etcd:v3.x.y
1. service_broker_mariadb_galera (当前项目镜像)
1. mariadb:10.2-custom （此镜像地址将配置在service_broker_mariadb_galera程序MARIADBIMAGE环境变量中）
1. phpmyadmin:4.6 （此镜像地址将配置在service_broker_mariadb_galera程序PHPMYADMINIMAGE环境变量中）
1. (或许需要) Prometheus 

### (k8s集群管理员) 给k8s集群中运行MySQL pods的三个nodes打label

所有MariaDB pod将只会起在这三个node上。

Example:
```
kubectl label nodes node-name-0 asiainfo-operator-mariadb-galera=host-path
kubectl label nodes node-name-1 asiainfo-operator-mariadb-galera=host-path
kubectl label nodes node-name-2 asiainfo-operator-mariadb-galera=host-path
```

此标签将设置在MARIADBGALERAHOSTPATHNODELABELS环境变量中。

### (k8s集群管理员) 在k8s集群中创建一个namespace

service_broker_mariadb_galera程序和所有生成的MySQL集群资源将创建在此namespace中。

此namespace将被配置在service_broker_mariadb_galera程序SBNAMESPACE环境变量中。

### (k8s集群管理员) 在上述namespace中创建一个具有edit角色的ServiceAccount

创建ServiceAccount范例：
```
oc create serviceaccount rescreator
```

或者从yaml创建：
```
apiVersion: v1
kind: ServiceAccount
metadata:
  name: service-brokers
  namespace: rescreator
```

为此ServiceAccount添加role的yaml范例1（需要查一下apiVersion）：
```
apiVersion: rbac.authorization.k8s.io/v1alpha1
kind: RoleBinding
metadata:
  name: edit
  namespace: service-brokers
roleRef:
  name: edit
subjects:
- kind: ServiceAccount
  name: rescreator
  namespace: service-brokers
userNames:
- system:serviceaccount:service-brokers:rescreator
```

为此ServiceAccount添加role的yaml范例2：
```
apiVersion: rbac.authorization.k8s.io/v1alpha1
kind: ClusterRoleBinding
metadata:
  name: rescreator
  namespace: ""
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
- kind: ServiceAccount
  name: rescreator
  namespace: service-brokers
```

service_broker_mariadb_galera程序将使用此AccountService的token。
此token的base64加密形式可以从其对应secret中提取。
使用`echo <token> | base64 -d`命令可以获取此token。

此token将被配置在service_broker_mariadb_galera程序SERVICE_ACCOUNT_TOKEN环境变量中。

### (k8s集群管理员) 创建ETCD程序

ETCD的信息应配置在以下几个环境变量中：
```
ETCDENDPOINT: http://xxx:2379
ETCDUSER: xxx
ETCDPASSWORD: xxx
```

需要挂载一个外部卷。

(todo: container command line)

ETCD的初始化脚本可以选配在service_broker_mariadb_galera程序ETCD_REGISTRY_INIT_INFO环境变量中：格式为init_script_file:api_user:api_password

比如：
```
servicebroker/mysql_galera_hostpath/install/etcd.sh:asiainfoLDP:2016asia
```

当以后调用service_broker_mariadb_galera程序的API的时候，api_user:api_password需要最为basic auth参数传入。

### (k8s集群管理员) 创建service_broker_mariadb_galera程序

注意设置上面提到的环境变量。

出了以上环境变量，还需设置以下环境变量：
* OPENSHIFTADDR: k8s master (api server)地址。比如https://10.1.3.2333:443
* BROKERPORT: service_broker_mariadb_galera程序的容器内部的监听端口。

以下环境变量也需要设置，但MySQL hostpath plan并不需要，因此可以将它们的值设为xxx。
* OPENSHIFTUSER
* OPENSHIFTPASS
* ENDPOINTSUFFIX
* STORAGECLASSNAME
* DNSMASQ_SERVER
* NODE_ADDRESSES
* NODE_DOMAINS
* EXTERNALZOOKEEPERSERVERS

# 创建MySQL实例

### 需在三个k8s node上预先创建好同名mysql data目录

此data目录将在调用operator的provision API的时候做为参数(datapath)传入。
详见下面的curl范例。

### curl范例

```
curl -i -X PUT http://apiuser:apipass@operator-address/v2/service_instances/mysql-host-path-test -d '{
  "service_id":"0f96b0f0-6a25-4018-8225-8f1cd090b1f9",
  "plan_id":"b80b0b7d-5108-4038-b560-67d82e6a43b7",
  "organization_guid": "default",
  "space_guid":"space-guid",
  "accepts_incomplete":true,
  "parameters": {"datapath": "/var/lib/mysql"}
}' -H "Content-Type: application/json"

curl -i -X DELETE -L 'http://apiuser:apipass@operator-address/v2/service_instances/mysql-host-path-test?service_id=0f96b0f0-6a25-4018-8225-8f1cd090b1f9&plan_id=b80b0b7d-5108-4038-b560-67d82e6a43b7'
```












