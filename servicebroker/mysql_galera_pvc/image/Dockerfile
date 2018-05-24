FROM registry.new.dataos.io/service-brokers/mariadb:10.2.12
# FROM registry.new.dataos.io/wangyd/mariadb:v4

COPY privileges.sh /usr/local/bin/
COPY custom-run.sh /usr/local/bin/

ENTRYPOINT ["/usr/local/bin/custom-run.sh"]

USER "mysql"
