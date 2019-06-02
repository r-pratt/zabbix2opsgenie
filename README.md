![Coverage](img/coverage_badge.png)

Compatible with Opsgenie's generic API integration (do not use this with the "built-in" Zabbix integration) and/or Edge Encryption. 


Other changes:
  - Priority Mapping
  - Config is independent of Marid/OEC
  - Logs to ```/etc/opsgenie/zabbix2opsgenie.log``` by default because of OEC log directory permissions.
  
[Config path:](https://github.com/r-pratt/zabbix2opsgenie/blob/master/zabbix2opsgenie.go#L27) ```confPath```

[Log path:](https://github.com/r-pratt/zabbix2opsgenie/blob/master/zabbix2opsgenie.go#L28) ```logPath```


# Install

Using Go:

```bash
go get github.com/r-pratt/zabbix2opsgenie

ln -s $GOPATH/bin/zabbix2opsgenie /etc/opsgenie/zabbix2opsgenie

ln -s $GOPATH/src/github.com/r-pratt/zabbix2opsgenie/zabbix2opsgenie.json /etc/opsgenie/zabbix2opsgenie.json
```

Without Go:

```bash
mkdir /etc/opsgenie && cd /etc/opsgenie

curl -O 'https://raw.githubusercontent.com/r-pratt/zabbix2opsgenie/master/zabbix2opsgenie'

curl -O 'https://raw.githubusercontent.com/r-pratt/zabbix2opsgenie/master/zabbix2opsgenie.json'

chmod +x zabbix2opsgneie
```

[Zabbix Action Command:](https://raw.githubusercontent.com/opsgenie/opsgenie-integration/master/zabbixIncoming/zabbix/actionCommand.txt)


```
/etc/opsgenie/zabbix2opsgenie -triggerName='{TRIGGER.NAME}' -triggerId='{TRIGGER.ID}' -triggerStatus='{TRIGGER.STATUS}' -triggerSeverity='{TRIGGER.SEVERITY}' -triggerDescription='{TRIGGER.DESCRIPTION}' -triggerUrl='{TRIGGER.URL}' -triggerValue='{TRIGGER.VALUE}' -triggerHostGroupName='{TRIGGER.HOSTGROUP.NAME}' -hostName='{HOST.NAME}' -ipAddress='{IPADDRESS}' -eventId='{EVENT.ID}' -date='{DATE}' -time='{TIME}' -itemKey='{ITEM.KEY}' -itemValue='{ITEM.VALUE}' -recoveryEventStatus='{EVENT.RECOVERY.STATUS}'
```
