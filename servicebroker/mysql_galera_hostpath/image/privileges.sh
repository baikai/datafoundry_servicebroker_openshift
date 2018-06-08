#!/bin/bash

sleep 3

mysql -uroot -p${MYSQL_ROOT_PASSWORD} -e "use mysql; update user set host = '%' where user = 'root'; flush privileges; select host, user from user;"

while [ $? -ne 0 ]; do
	echo "[#] modify privileges"
	sleep 3
	mysql -uroot -p${MYSQL_ROOT_PASSWORD} -e "use mysql; update user set host = '%' where user = 'root'; flush privileges; select host, user from user;"
done
