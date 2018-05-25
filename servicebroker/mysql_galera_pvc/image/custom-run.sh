#!/bin/sh

/usr/local/bin/privileges.sh &

# mkdir -p /var/lib/mysql/data/db

echo Start mysql server

/usr/local/bin/docker-entrypoint.sh mysqld
