FROM registry.new.dataos.io/service-brokers/mariadb:10.2.12

USER "mysql"

COPY privileges.sh /usr/local/bin/
COPY custom-run.sh /usr/local/bin/

CMD ["/usr/local/bin/custom-run.sh"]
