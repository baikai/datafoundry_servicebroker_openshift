#!/bin/bash

sleep 5

mysql -uroot -p{{ .RootPassword }} -e "use mysql; update user set host = '%' where user = 'root'; flush privileges; select host, user from user;"

