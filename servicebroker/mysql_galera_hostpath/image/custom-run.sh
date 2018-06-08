#!/bin/sh

mkdir -p /mysqlparent/instance-$INSTANCE_ID

/usr/local/bin/privileges.sh &

echo Start mysql server

/usr/local/bin/docker-entrypoint.sh mysqld
