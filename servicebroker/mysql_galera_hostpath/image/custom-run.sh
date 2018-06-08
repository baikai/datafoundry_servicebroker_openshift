#!/bin/sh


# mkdir -p /mysqlparent/instance-$INSTANCE_ID
# chown -R mysql:mysql /mysqlparent/instance-$INSTANCE_ID
# chmod 777 /mysqlparent/instance-$INSTANCE_ID

/usr/local/bin/privileges.sh &

echo Start mysql server

/usr/local/bin/docker-entrypoint.sh mysqld
