# Written by Jared on Feb 27, 2018.
# Script to set configurations for elasticsearch.
set -x
# get ip address firstly
while [ "${ip_addr}"x = ""x ] 
do
  ip_addr=$(getent hosts $(hostname).${SRVNAME} | cut -d' ' -f1)
done

names=$(getent hosts $(hostname).${SRVNAME} | awk -F' ' '{print $NF}' | cut -d'.' -f2-6)

sed -i "s/NETWORKHOST/${ip_addr}/g" /usr/share/elasticsearch/config/elasticsearch.yml

sed -i "s/cluster-name/${CLUSTER_NAME}/g" /usr/share/elasticsearch/config/elasticsearch.yml

echo "discovery.zen.ping.unicast.hosts:" >> /usr/share/elasticsearch/config/elasticsearch.yml

# get current ordinal
PREFIX=$(hostname | cut -d'-' -f-2)
ordinal=$(hostname | cut -d'-' -f3)
ordinal=$(( $ordinal + 1))
i=0
if [ $ordinal -gt ${NODES_NUM} ]; then
  NODES_NUM=$ordinal
fi

while [ $i -lt ${NODES_NUM} ]
do
  echo " - "${PREFIX}"-"${i}.${names} >> /usr/share/elasticsearch/config/elasticsearch.yml
  i=$(($i+1))
done
