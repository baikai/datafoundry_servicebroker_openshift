#!/bin/bash

ETCD_USER=root
ETCD_PASSWORD=rootpasswd
ETCD_ADDRESS=http://localhost:2379

API_NAME=asiainfo
API_PASSWORD=MysqlGalera

# export ETCDCTL="etcdctl --timeout 15s --total-timeout 30s --endpoints $ETCD_ADDRESS --username $ETCD_USER:$ETCD_PASSWORD"
export ETCDCTL="etcdctl --timeout 15s --total-timeout 30s --endpoints $ETCD_ADDRESS"

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


