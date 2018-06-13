

kubernetes auth: through service account (need create priledge).


etcd cluster in k8s (deplyment.yaml)
* etcd 3
* auto init plan infos
* https://github.com/coreos/etcd/blob/master/Documentation/op-guide/clustering.md
* https://github.com/coreos/etcd-operator/blob/master/doc/user/install_guide.md (need k8s 1.8+)

need a mysql monitor (with web frontend).
* https://github.com/bmildren/docker-mysql-monitoring
* https://hub.docker.com/r/logzio/mysql-monitor/
* https://gitee.com/ruzuojun/Lepus
* https://github.com/prometheus/mysqld_exporter

https://mariadb.com/kb/en/library/galera-cluster-system-variables/#wsrep_max_ws_rows: no limits in fact.
https://mariadb.com/kb/en/library/galera-cluster-system-variables/#wsrep_max_ws_size: 2GB
* This restriction will probably be fixed in Galera 4.0. http://www.fromdual.com/limitations-of-galera-cluster

# 安装 operator_mariadb_galera

### (k8s集群管理员) 上传镜像

将两个镜像上传放置到合适位置（须保证k8s集群可以拉取）：
1. mariadb:10.2-custom-v21 （此镜像地址将配置在operator_mariadb_galera程序环境变量中）
1. phpmyadmin:4.6 （此镜像地址将配置在operator_mariadb_galera程序环境变量中）
1. etcd:v3.x.y
1. operator_mariadb_galera:latest (当前项目镜像)

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

operator_mariadb_galera程序和所有生成的MySQL集群资源将创建在此namespace中。

此namespace将被配置在SBNAMESPACE环境变量中。

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



operator_mariadb_galera程序将使用此AccountService的token。
此token的base64加密形式可以从其对应secret中提取。
使用`echo <token> | base64 -d`命令可以获取此token。

此token将被配置在SERVICE_ACCOUNT_TOKEN环境变量中。



### (k8s集群管理员) 创建ETCD程序

ETCD的信息应配置在以下几个环境变量中：
```
ETCDENDPOINT: http://xxx:2379
ETCDUSER: xxx
ETCDPASSWORD: xxx
```

ETCD的初始化脚本可以选配在环境变量ETCD_REGISTRY_INIT_INFO中：格式为init_script_file:api_user:api_password

比如：
```
servicebroker/mysql_galera_hostpath/install/etcd.sh:asiainfoLDP:2016asia
```

### (k8s集群管理员) 创建operator_mariadb_galera程序

注意设置上面提到的环境变量。


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
  "parameters": {"ami_id":"ami-ecb68a84", "datapath": "/var/lib/mysql"}
}' -H "Content-Type: application/json"

curl -i -X DELETE -L 'http://apiuser:apipass@operator-address/v2/service_instances/mysql-host-path-test?service_id=0f96b0f0-6a25-4018-8225-8f1cd090b1f9&plan_id=b80b0b7d-5108-4038-b560-67d82e6a43b7'
```












