# Added by Jared on March 29, 2018.

It seemed that image provided by elasticsearch had issue about log dir.

path.log doesn't take effect. Only /usr/share/elasticsearch/logs/ could be used for storing logs and mounted external volume.

Elastic search cluster could be deployed with statefulset.

It could be deployed on kubernetes version later than 1.6.

Persistent Volume could be allocated by dynamic storage class.
