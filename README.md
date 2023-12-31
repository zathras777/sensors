# sensors
Go daemon to provide sensor data from a RaspberryPi with a number of USB and directly connected devices to a HomeAssistant instance.

## Configuration
The configuration is done via a yaml file.

```yaml
http:
  address: 127.0.0.1
  port: 7001

zcan:
  - name: mvhr
    interface: can0
    nodeid: 50
    pdo:
      node: 1
      pdo:
        - slug: avoided_heating_actual
          interval: 10
      ...
modbus:
  - name: t300
    slaveid: 20
    baudrate: 19200
    device: /dev/ttyUSB0
    interval: 5
    registers:
      holding:
        - description: "Heat Rod/Boost"
          tag: "C"
          register: 1
          typ: "u16"
        ...
      input:
        - description: "Temperature Before Evaporation"
          tag: "T05"
          register: 11
          typ: "s16"
          factor: 1
          offset: -100
        - description: "E-Valve Temperature"
          tag: T14
          register: 29
          offset: -100
          factor: 1
          typ: s16
        ...

max6675:
  - name: Water Temp
    path: /dev/spidev0.2
    interval: 10

```

For modbus register entries, the factor is powers of 10, e.g. a raw value of 489 with a factor of 1 will result in 48.9 being returned.

Once configured, the server is started with the filename of the configuration file. If no file is provided then the default of config.yaml in the same directory will be looked for.

## Output

The server creates a simple webserver that serves data from all configured services. The URL is simply the name of the server, all lowercase and with spaces replaced by _. A zcan device will also provide a device-info endpoint.


```logfile

2023/11/01 17:09:48 Started HTTP Sensors Service.
2023/11/01 17:09:48 Starting network services
2023/11/01 17:09:48 zcan service mvhr setup OK
2023/11/01 17:09:48 modbus service t300 setup OK
2023/11/01 17:09:48 MAX6675 service RecircTemp setup OK
2023/11/01 17:09:48 Trying to start HTTP server listening @ http://10.0.xxx.xxx:7001/
2023/11/01 17:09:48 available endpoints: /mvhr, /mvhr/device-info, /recirctemp, /t300
2023/11/01 17:09:49 unknown sensor 0x1a2 [418] 1 bytes of data
```

The data is served as JSON.

```shell
GET /t300 HTTP/1.0

HTTP/1.0 200 OK
Content-Type: application/json
Date: Sat, 14 Oct 2023 18:40:01 GMT
Content-Length: 181

{"A":1,...
```

## zcan Requirements
The zcan sensor uses the linux socketcan interface to read/write to the device. This needs to have the bitrate set and the interface brought UP - both of which need root level access. If using this sensor then the app needs to be run as root.

## ToDo
- add logging options
- expand the modbus options available
- add more sensors
- look at running as non-root

## Thanks
Much of the zcan code here is inspired and shaped by the work done here https://github.com/michaelarnauts/aiocomfoconnect

## Feedback Welcome!
This can probably be improved on in many ways, but it's working and is more stable than the python apps it replaces. Happy to have patches or feedback... :-)