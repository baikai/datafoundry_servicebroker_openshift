

现状:
* 一般开若干个MySQL　HA集群，由有经验的DBA手动规划哪些表存储在哪些集群中。
* 运维部署都很繁琐，容易出错。

# 几种方案

目前有很多中间件方案，底层为原生MySQL非分布式MySQL Server。
下面只考察了名声比较大的MyCat和Vitess。
另外还考察了仿照Google Spanner实现的兼容MySQL协议的TiDB。

### MyCat

数据库中间件。社区维护开发。

官网: http://mycat.io

项目地址: https://github.com/MyCATApache/Mycat-Server/wiki

架构图参考:
* http://www.cnblogs.com/raphael5200/p/5893472.html
* http://soft.dog/2016/03/05/mycat-ha-lb/

优点
* 后端服务器采用原生MySQL Server。
* 网上有一些成功使用案例报告。

缺点
* mycat中间件服务器是Java写的，内存消耗不乐观
* 需要的各种开源组件太多，部署繁琐。
* 配置文件非常多，官方文档非常不全，很多配置都语焉不详。官网首页很多连接都已失效，给人一种久未维护的感觉。
* 使用配置文件手动分库分表，需要经验。
* 这是一个社区项目，没有专业公司支持。
* 有用户报告有丢数据的情况。
* 部分SQL语句的使用有限制。
* 扩缩容必须离线进行，并且比较繁琐。
* 周边工具比较欠缺。

### Vitess

数据库中间件。Youtube工程师维护开发。

官网: https://vitess.io

项目地址: https://github.com/vitessio/vitess

架构图: https://vitess.io/overview/#architecture

优点:
* 后端服务器采用原生MySQL Server。
* youtube工程师维护
* 用在youtube和slack生产上
* 官方网站有在kubernetes上部署的文档。

缺点
* 使用配置文件手动分库分表，需要经验。
* 有不少运维概念需要消化理解。
* Google并没有大力支持。参与的工程师并不多。

### TiDB

兼容MySQL协议。整个体系基本从头实现。创业公司PingCAP维护开发。

官网: https://pingcap.com/en/

项目地址: https://github.com/pingcap/tidb

架构图: https://pingcap.com/docs/overview/#tidb-architecture

优点
* 有专业公司支持开发，不断维护和改进。公司为创业公司，比较再一自己的口碑。
* 文档比较全面清晰。https://pingcap.github.io/docs/
* 部署相对简单。
* 自动分表。
* 支持分布式事务(transaction)和分布式Join。
* 各种工具程序（监控，迁移等）都有专人维护。
* 感觉比较乐于迎合各种新技术，比如k8s和promixius
* 已经有不少成功使用案例。https://zhuanlan.zhihu.com/newsql

缺点
* 后端采用RocksDB而非MySQL Server，因此不能和MySQL协议100%兼容，但可以做到95%兼容。
  对于大多数项目问题不大。
  https://github.com/pingcap/docs/blob/master/sql/mysql-compatibility.md
* 对于事务（transaction）十分频繁的情况需要程序做些处理，比如将多个transactions合并为一个提交。
  对于大多数项目不会遇到这种情况。
* 项目较新，去年1.0。目前2.0即将发布。但目前官方水文显示口碑不错。
* 某些周边运维工具以后可能可能收费。但核心组件永久开源免费。

### MariaDB Galera Cluster

(高可用，多master，但非分布式)

官网: https://mariadb.com/kb/en/library/galera-cluster/

架构图: https://blog.csdn.net/chinagissoft/article/details/46964421

优点
* 有专业公司支持开发。
* 文档比较全面清晰。
* 部署相对简单。

缺点
* 不支持数据分布。







