# base image for elastic search 6.2
FROM docker.elastic.co/elasticsearch/elasticsearch-oss:6.2.2

COPY elasticsearch.yml /usr/share/elasticsearch/config/elasticsearch.yml
COPY config.sh /usr/share/elasticsearch/bin/config.sh
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh

RUN chmod g+rw /usr/share/elasticsearch/config/elasticsearch.yml \
  && chmod +x /usr/share/elasticsearch/bin/config.sh \
  && chown 1000 /usr/share/elasticsearch/bin/config.sh \
  && chown 1000 /usr/local/bin/docker-entrypoint.sh \
  && chmod 0775 /usr/local/bin/docker-entrypoint.sh \
  && echo "*  soft nofile 65535" >> /etc/security/limits.conf \
  && echo "*  hard nofile 65535" >> /etc/security/limits.conf \
  && echo "*  soft nproc  65535" >> /etc/security/limits.conf \
  && echo "*  hard nproc  65535" >> /etc/security/limits.conf

# In order to meet the requirement for vm.max_map_count = 261244, setting it
# in /etc/sysctl.conf on host to make it take effect

USER 1000
