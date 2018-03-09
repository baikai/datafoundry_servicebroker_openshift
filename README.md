

## Supported API Parameters For `Storm_external_standalone` Plan

* `krb5conf`: content of `krb5.conf`, must be base64 encoded.
* `kafkaclient-keytab`: content of `storm.keytab`, must be base64 encoded.
* `kafkaclient-service-name`: value of `serviceName` for Kafka client in `storm_jaas.conf`.
* `kafkaclient-principal`: value of `principal` for kafka client in `storm_jaas.conf`.

To enable kerberos support on storm instance, all of 4 parameters above must be set, otherwise kerberos support will be disabled.

### example of bsi.yaml

```yaml
apiVersion: v1
kind: BackingServiceInstance
metadata:
  annotations:
    datafoundry.io/servicebroker: etcd
  name: storm-kerberos
spec:
  provisioning:
    backingservice_name: Storm
    backingservice_plan_guid: PLAN_ID_OF_EXTERNAL_STANDALONE
    backingservice_plan_name: external_standalone
    backingservice_spec_id: SERVICE_ID_OF_STORM
    parameters:
      memory: "2"
      kafkaclient-service-name: ocdp
      kafkaclient-principal: ocdp-clusteraa@ASIAINFO.COM
      kafkaclient-keytab: BQIAAABFAAEADEFTSUFJTkZPLkNPTQAOb2NkcC1jbHVzdGVyYWEAAAABWlM1cgEAEAAYeukOa3W1E3raUjF2dT1n/Z5whqhr+Df4AAAANQABAAxBU0lBSU5GTy5DT00ADm9jZHAtY2x1c3RlcmFhAAAAAVpTNXIBAAMACNCGnXq808ccAAAATQABAAxBU0lBSU5GTy5DT00ADm9jZHAtY2x1c3RlcmFhAAAAAVpTNXIBABIAIOM/D8Yy5DsHwNoEBd9BhAfD5f2gqYHUOFDXpf9c5fSxAAAAPQABAAxBU0lBSU5GTy5DT00ADm9jZHAtY2x1c3RlcmFhAAAAAVpTNXIBABcAEOJ9L4ngHCiZNVH1I50R5bUAAAA9AAEADEFTSUFJTkZPLkNPTQAOb2NkcC1jbHVzdGVyYWEAAAABWlM1cgEAEQAQdHmDBqZ45tAjQsBgOG8yXA==
      krb5conf: W2xpYmRlZmF1bHRzXQogIHJlbmV3X2xpZmV0aW1lID0gN2QKICBmb3J3YXJkYWJsZSA9IHRydWUKICBkZWZhdWx0X3JlYWxtID0gQVNJQUlORk8uQ09NCiAgdGlja2V0X2xpZmV0aW1lID0gMjRoCiAgZG5zX2xvb2t1cF9yZWFsbSA9IGZhbHNlCiAgZG5zX2xvb2t1cF9rZGMgPSBmYWxzZQogIGRlZmF1bHRfY2NhY2hlX25hbWUgPSAvdG1wL2tyYjVjY18le3VpZH0KICAjZGVmYXVsdF90Z3NfZW5jdHlwZXMgPSBhZXMgZGVzMy1jYmMtc2hhMSByYzQgZGVzLWNiYy1tZDUKICAjZGVmYXVsdF90a3RfZW5jdHlwZXMgPSBhZXMgZGVzMy1jYmMtc2hhMSByYzQgZGVzLWNiYy1tZDUKCltsb2dnaW5nXQogIGRlZmF1bHQgPSBGSUxFOi92YXIvbG9nL2tyYjVrZGMubG9nCiAgYWRtaW5fc2VydmVyID0gRklMRTovdmFyL2xvZy9rYWRtaW5kLmxvZwogIGtkYyA9IEZJTEU6L3Zhci9sb2cva3JiNWtkYy5sb2cKCltyZWFsbXNdCiAgQVNJQUlORk8uQ09NID0gewogICAgYWRtaW5fc2VydmVyID0gMTAuMS4yMzYuMTQ2CiAgICBrZGMgPSAxMC4xLjIzNi4xNDYKICB9
```


