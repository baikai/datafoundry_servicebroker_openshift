

=========================== mysql host path


curl -i -X PUT http://asiainfo:MysqlGalera@servicebroker-openshift.lo.dataos.io/v2/service_instances/mysql-host-path-test -d '{
  "service_id":"0f96b0f0-6a25-4018-8225-8f1cd090b1f9",
  "plan_id":"b80b0b7d-5108-4038-b560-67d82e6a43b7",
  "organization_guid": "default",
  "space_guid":"space-guid",
  "accepts_incomplete":true,
  "parameters": {"ami_id":"ami-ecb68a84", "datapath": "/var/lib/mysql"}
}' -H "Content-Type: application/json"



curl -i -X DELETE -L 'http://asiainfo:MysqlGalera@servicebroker-openshift.lo.dataos.io/v2/service_instances/mysql-host-path-test?service_id=0f96b0f0-6a25-4018-8225-8f1cd090b1f9&plan_id=b80b0b7d-5108-4038-b560-67d82e6a43b7'




