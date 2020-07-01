# yotta-miner-tracker

## 1. 部署与配置：

在项目的main目录下编译：
```
$ go build -o minertracker
```
配置文件为`yotta-miner-tracker.yaml`（项目的main目录下有示例），默认可以放在`home`目录或抽查程序同目录下，各配置项说明如下：
```
#HTTP服务端口，默认值为8080
http-bind-addr: ":8080"
#EOS服务地址，默认值为http://127.0.0.1:8888
eos-url: "http://127.0.0.1:8888"
#MongoDB数据库地址，默认值为mongodb://127.0.0.1:27017/?connect=direct
mongodb-url: "mongodb://127.0.0.1:27017/?connect=direct"
#消息队列配置
auramq:
  #服务端配置
  server:
    #MQ服务端口，默认值为8787
    bind-addr: ":8787"
    #消息路由的缓冲区长度，默认值为8192
    router-buffer-size: 8192
    #订阅者缓冲区长度，默认值为1024
    subscriber-buffer-size: 1024
    #读缓存大小，默认值为4096字节
    read-buffer-size: 4096
    #写缓存大小，默认值为4096字节
    write-buffer-size: 4096
    #ping超时时间，默认值为30（秒）
    ping-wait: 30
    #读超时时间，默认值为60（秒）
    read-wait: 60
    #写超时时间，默认值为10（秒）
    write-wait: 10
    #发布消息的队列名称，默认值为sync
    miner-sync-topic: "sync"
  #客户端配置
  client:
    #订阅者缓冲区长度，默认值为1024
    subscriber-buffer-size: 1024
    #ping超时时间，默认值为30（秒）
    ping-wait: 30
    #读超时时间，默认值为60（秒）
    read-wait: 60
    #写超时时间，默认值为10（秒）
    write-wait: 10
    #监听消息的队列名称，默认值为sync
    miner-sync-topic: "sync"
    #要连接的全部MQ地址，默认值为空
    all-sn-urls:
    - "ws://172.17.0.2:8787/ws"
    - "ws://172.17.0.3:8787/ws"
    - "ws://172.17.0.4:8787/ws"
    - "ws://172.17.0.5:8787/ws"
    - "ws://172.17.0.6:8787/ws"
    #鉴权用BP账号，默认值为空
    account: "yottanalysis"
    #鉴权用账号的私钥，默认值为空
    private-key: "5JdrCwfnPcqFH8osGqSy52WbcSB93wc3BLWXnSDdJZ3ffyie4HT"
    #客户端标识ID，默认值为yottaminertracker
    client-id: "yottaminertracker"
#日志配置
logger:
  #日志输出类型：stdout为输出到标准输出流，file为输出到文件，默认为stdout，此时只有level属性起作用，其他属性会被忽略
  output: "file"
  #日志路径，默认值为./tracker.log，仅在output=file时有效
  file-path: "./tracker.log"
  #日志拆分间隔时间，默认为24（小时），仅在output=file时有效
  rotation-time: 24
  #日志最大保留时间，默认为240（10天），仅在output=file时有效
  max-age: 240
  #日志输出等级，默认为Info
  level: "Debug"
#其他设置
misc:
  #授权账号表的刷新时间，默认为600（秒）
  refresh-auth-interval: 600

```
启动服务：
```
$ nohup ./minertracker &
```
## 2. 数据库配置：
需在mongoDB中建立名为`minertracker`的数据库，其包含两张表：`Auth`和`Node`，其中`Auth`表用于记录的是鉴权账号，这些账号用于第三方服务接入消息队列时的鉴权，结构如下：

| 字段 | 类型 | 描述 |
| ---- | ---- | ---- |
| _id | string | 账号名，需在BP中存在，主键 |
| publickey | string | 账号所属active公钥 |

默认只需要填入`_id`对应的账号名即可，程序会自动从BP获取公钥写入数据库。注意，服务启动前需要将配置文件中`auramq.client.account`对应的账号录入数据库。
`Node`表记录的是从SN同步过来的矿机数据，其结构与SN数据库的`yotta.Node`表相同，服务启动前需要先将SN中全部矿机数据导入该表：
在SN端：
```
$ mongoexport -h 127.0.0.1 --port 27017 -d yotta -c Node -o node.json
```
在分析服务器端：
```
$ mongoimport -h 127.0.0.1 --port 27017 -d minertracker -c Node --file node.json
```

## 3. 查询数据
服务启动后会使用配置文件中`http-bind-addr`参数指定的端口对外提供基于HTTP协议的矿机信息查询服务，比如查询全部矿机，可以使用下边命令（假设服务位于本机的8080端口）：
```
$ curl -XPOST http://127.0.0.1:8080/query
```

查询ID为17的矿机：
```
$ curl -XPOST -d'{"_id": 17}' http://127.0.0.1:8080/query
```
查询上报时间大于某时间的矿机：
```
$ curl -XPOST -d'{"timestamp": {"$gt": 1593598279}}' http://127.0.0.1:8080/query
```
POST请求体为查询条件（JSON格式的mongodb查询字符串），查询成功后返回矿机信息的JSON数组