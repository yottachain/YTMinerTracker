package main

import (
	"fmt"

	"github.com/aurawing/auramq"
	"github.com/aurawing/auramq/msg"
	wsclient "github.com/aurawing/auramq/ws/cli"
	"github.com/golang/protobuf/proto"
	ytcrypto "github.com/yottachain/YTCrypto"
	yttracker "github.com/yottachain/yotta-miner-tracker"
	pb "github.com/yottachain/yotta-miner-tracker/pbtracker"
)

func main() {
	wsurl := "ws://192.168.36.132:8787/ws"                              //MQ连接端口
	data := []byte("12345678")                                          //用于鉴权的数据，最好随机生成
	account := "testbpaccount"                                          //鉴权用BP账号，需在BP存在且注册到服务端MQ服务数据库的Auth表中
	privatekey := "5JdrCwfnPcqFH8osGqSy52WbcSB93wc3BLWXnSDdJZ3ffyie4HT" //鉴权账号对应的私钥
	signature, err := ytcrypto.Sign(privatekey, data)                   //对鉴权数据签名
	if err != nil {
		panic(err)
	}
	sigMsg := &pb.SignMessage{AccountName: account, Data: data, Signature: signature} //构造鉴权消息
	crendData, err := proto.Marshal(sigMsg)                                           //对消息序列化
	if err != nil {
		panic(err)
	}
	//连接MQ，callback为消息回调函数，msg.AuthReq的Id属性需唯一标明客户端ID，不可重复，sync为要监听的队列名称，1024为客户端消息缓存大小，30为客户端ping超时时间，60为读超市时间，10为写超时时间，单位均为秒，这三个值最好不动
	cli, err := wsclient.Connect(wsurl, callback, &msg.AuthReq{Id: "testclient", Credential: crendData}, []string{"sync"}, 1024, 30, 60, 10)
	if err != nil {
		panic(err)
	}
	//开始监听
	cli.Run()
}

func callback(msg *msg.Message) {
	//判断消息是否为广播消息且为要监听的队列
	if msg.GetType() == auramq.BROADCAST && msg.GetDestination() == "sync" {
		nodemsg := new(pb.NodeMsg)
		err := proto.Unmarshal(msg.Content, nodemsg) //反序列化消息
		if err != nil {
			fmt.Println("decoding nodeMsg failed")
			return
		}
		node := new(yttracker.Node)
		err = node.Fillby(nodemsg) //将消息转换回Node结构体
		if err != nil {
			fmt.Println("convert protobuf message to node")
			return
		}
		fmt.Printf("received node %d\n", node.ID)
	}
}
