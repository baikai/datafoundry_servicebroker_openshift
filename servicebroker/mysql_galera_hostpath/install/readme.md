

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

```
# operator_mariadb_galera.yaml
```

(k8s集群管理员) 运行:
```
kubectl create -f operator_mariadb_galera.yaml
```

### route ?

nodeport

# 初始化 operator_mariadb_galera 程序

export OPERATOR_MARIADB_GALERA_ADDR=http://xxxx.xxx

curl -X PUT $OPERATOR_MARIADB_GALERA_ADDR/init
curl -X GET $OPERATOR_MARIADB_GALERA_ADDR/info

# 通过 operator_mariadb_galera 程序创建新的 mariadb galera 集群服务实例

export OPERATOR_MARIADB_GALERA_ADDR=http://xxxx.xxx

curl -X PUT $OPERATOR_MARIADB_GALERA_ADDR/api/v1/create-instance
curl -X DELETE $OPERATOR_MARIADB_GALERA_ADDR/api/v1/delete-instance
curl -X GET $OPERATOR_MARIADB_GALERA_ADDR/api/v1/query-instance




