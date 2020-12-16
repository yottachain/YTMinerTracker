package yttracker

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/aurawing/auramq"
	"github.com/aurawing/auramq/embed"
	ebclient "github.com/aurawing/auramq/embed/cli"
	"github.com/aurawing/auramq/msg"
	"github.com/aurawing/auramq/ws"
	wsclient "github.com/aurawing/auramq/ws/cli"
	"github.com/aurawing/eos-go"
	"github.com/golang/protobuf/proto"
	log "github.com/sirupsen/logrus"
	ytcrypto "github.com/yottachain/YTCrypto"
	pb "github.com/yottachain/yotta-miner-tracker/pbtracker"
)

//Service sync service
type Service struct {
	client   auramq.Client
	mongoCli *mongo.Client
}

//StartSync start syncing
func StartSync(api *eos.API, mongoCli *mongo.Client, serverConf *ServerConfig, clientConf *ClientConfig, miscConf *MiscConfig) (*Service, error) {
	entry := log.WithFields(log.Fields{Function: "StartSync"})
	refreshAuth(api, mongoCli)
	go func() {
		for {
			time.Sleep(time.Duration(miscConf.RefreshAuthInterval) * time.Second)
			refreshAuth(api, mongoCli)
		}
	}()

	syncService := new(Service)
	syncService.mongoCli = mongoCli
	router := auramq.NewRouter(serverConf.RouterBufferSize)
	go router.Run()
	wsbroker := ws.NewBroker(router, serverConf.BindAddr, true, syncService.auth, serverConf.SubscriberBufferSize, serverConf.ReadBufferSize, serverConf.WriteBufferSize, serverConf.PingWait, serverConf.ReadWait, serverConf.WriteWait)
	wsbroker.Run()
	ebbroker := embed.NewBroker(router, true, syncService.auth, serverConf.SubscriberBufferSize, serverConf.SubscriberBufferSize, serverConf.SubscriberBufferSize)
	ebbroker.Run()
	entry.Info("MQ server created")

	data := []byte(getRandomString(8))
	signature, err := ytcrypto.Sign(clientConf.PrivateKey, data)
	if err != nil {
		entry.WithError(err).Error("generating signature")
		return nil, err
	}
	sigMsg := &pb.SignMessage{AccountName: clientConf.Account, Data: data, Signature: signature}
	crendData, err := proto.Marshal(sigMsg)
	if err != nil {
		entry.WithError(err).Error("encoding SignMessage")
		return nil, err
	}

	cli, err := ebclient.Connect(ebbroker.(*embed.Broker), func(msg *msg.Message) {}, &msg.AuthReq{Id: clientConf.ClientID, Credential: crendData}, []string{serverConf.MinerSyncTopic})
	if err != nil {
		entry.WithError(err).Error("connecting to embeded broker")
		return nil, err
	}
	go func() {
		cli.Run()
	}()
	entry.Info("create embeded broker successful")
	syncService.client = cli
	callback := func(msg *msg.Message) {
		if msg.GetType() == auramq.BROADCAST {
			if msg.GetDestination() == clientConf.MinerSyncTopic {
				nodemsg := new(pb.NodeMsg)
				err := proto.Unmarshal(msg.Content, nodemsg)
				if err != nil {
					entry.WithError(err).Error("decoding nodeMsg failed")
					return
				}
				node := new(Node)
				err = node.Fillby(nodemsg)
				if err != nil {
					entry.WithError(err).Error("convert protobuf message to node")
					return
				}
				syncNode(mongoCli, node, cli, serverConf.MinerSyncTopic)
			}
		}
	}

	for i, url := range clientConf.AllSNURLs {
		wsurl := url
		index := i
		go func() {
			for {
				cli, err := wsclient.Connect(wsurl, callback, &msg.AuthReq{Id: clientConf.ClientID, Credential: crendData}, []string{clientConf.MinerSyncTopic}, clientConf.SubscriberBufferSize, clientConf.PingWait, clientConf.ReadWait, clientConf.WriteWait)
				if err != nil {
					entry.WithError(err).Errorf("connecting to SN%d", index)
					time.Sleep(time.Duration(3) * time.Second)
					continue
				}
				entry.Infof("remote MQ server SN%d connected: %s", index, wsurl)
				cli.Run()
				entry.Infof("re-connect MQ SN%d server: %s", index, wsurl)
				time.Sleep(time.Duration(3) * time.Second)
			}
		}()
	}

	return syncService, nil
}

func syncNode(cli *mongo.Client, node *Node, mqcli auramq.Client, topic string) error {
	entry := log.WithFields(log.Fields{Function: "syncNode"})
	if node.ID == 0 {
		return errors.New("miner ID cannot be 0")
	}
	collection := cli.Database(MinerTrackerDB).Collection(NodeTab)
	if node.Uspaces == nil {
		node.Uspaces = make(map[string]int64)
	}
	_, err := collection.InsertOne(context.Background(), bson.M{"_id": node.ID, "nodeid": node.NodeID, "pubkey": node.PubKey, "owner": node.Owner, "profitAcc": node.ProfitAcc, "poolID": node.PoolID, "poolOwner": node.PoolOwner, "quota": node.Quota, "addrs": node.Addrs, "cpu": node.CPU, "memory": node.Memory, "bandwidth": node.Bandwidth, "maxDataSpace": node.MaxDataSpace, "assignedSpace": node.AssignedSpace, "productiveSpace": node.ProductiveSpace, "usedSpace": node.UsedSpace, "uspaces": node.Uspaces, "weight": node.Weight, "valid": node.Valid, "relay": node.Relay, "status": node.Status, "timestamp": node.Timestamp, "version": node.Version, "rebuilding": node.Rebuilding, "realSpace": node.RealSpace, "tx": node.Tx, "rx": node.Rx, "other": node.Other, "manualWeight": node.ManualWeight, "unreadable": node.Unreadable, "stableStat": &StableStatistics{StartTime: time.Now().Unix(), Counter: 0, Ratio: 1}})
	if err != nil {
		errstr := err.Error()
		if !strings.ContainsAny(errstr, "duplicate key error") {
			entry.WithError(err).Warnf("inserting miner %d to database", node.ID)
			return err
		}
		oldNode := new(Node)
		err := collection.FindOne(context.Background(), bson.M{"_id": node.ID}).Decode(oldNode)
		if err != nil {
			entry.WithError(err).Warnf("fetching miner %d", node.ID)
			return err
		}
		if oldNode.StableStat == nil {
			oldNode.StableStat = &StableStatistics{StartTime: time.Now().Unix(), Counter: 0, Ratio: 1}
		}
		newCounter := oldNode.StableStat.Counter + 1
		newRatio := float32(newCounter*60) / float32(time.Now().Unix()-oldNode.StableStat.StartTime)
		if newRatio > 1 {
			newRatio = 1
		}
		cond := bson.M{"nodeid": node.NodeID, "pubkey": node.PubKey, "owner": node.Owner, "profitAcc": node.ProfitAcc, "poolID": node.PoolID, "poolOwner": node.PoolOwner, "quota": node.Quota, "addrs": node.Addrs, "cpu": node.CPU, "memory": node.Memory, "bandwidth": node.Bandwidth, "maxDataSpace": node.MaxDataSpace, "assignedSpace": node.AssignedSpace, "productiveSpace": node.ProductiveSpace, "usedSpace": node.UsedSpace, "weight": node.Weight, "valid": node.Valid, "relay": node.Relay, "status": node.Status, "timestamp": node.Timestamp, "version": node.Version, "rebuilding": node.Rebuilding, "realSpace": node.RealSpace, "tx": node.Tx, "rx": node.Rx, "manualWeight": node.ManualWeight, "unreadable": node.Unreadable, "stableStat": &StableStatistics{StartTime: oldNode.StableStat.StartTime, Counter: newCounter, Ratio: newRatio}}
		if len(node.Other) > 0 {
			cond["other"] = node.Other
		}
		for k, v := range node.Uspaces {
			cond[fmt.Sprintf("uspaces.%s", k)] = v
		}
		opts := new(options.FindOneAndUpdateOptions)
		opts = opts.SetReturnDocument(options.After)
		result := collection.FindOneAndUpdate(context.Background(), bson.M{"_id": node.ID}, bson.M{"$set": cond}, opts)
		updatedNode := new(Node)
		err = result.Decode(updatedNode)
		if err != nil {
			entry.WithError(err).Warnf("updating record of miner %d", node.ID)
			return err
		}
		m, err := updatedNode.Convert()
		if err != nil {
			entry.WithError(err).Warnf("conver miner %d to protobuf message", node.ID)
			return err
		}
		if b, err := proto.Marshal(m); err != nil {
			entry.WithError(err).Errorf("marshal miner %d failed", updatedNode.ID)
		} else {
			mqcli.Publish(topic, b)
			entry.Debugf("publishing information of miner %d", updatedNode.ID)
		}
	}
	return nil
}

func (s *Service) auth(cred *msg.AuthReq) bool {
	entry := log.WithFields(log.Fields{Function: "auth"})
	signMsg := new(pb.SignMessage)
	err := proto.Unmarshal(cred.Credential, signMsg)
	if err != nil {
		entry.WithError(err).Error("decoding SignMessage")
		return false
	}
	collection := s.mongoCli.Database(MinerTrackerDB).Collection(AuthTab)
	auth := new(Auth)
	err = collection.FindOne(context.Background(), bson.M{"_id": signMsg.AccountName}).Decode(auth)
	if err != nil {
		entry.WithError(err).Errorf("decoding Auth record ofr account: %s", signMsg.AccountName)
		return false
	}

	if ytcrypto.Verify(auth.PublicKey, signMsg.Data, signMsg.Signature) {
		return true
	}
	return false
}

//Send one message to another client
func (s *Service) Send(to string, content []byte) bool {
	return s.client.Send(to, content)
}

//Publish one message
func (s *Service) Publish(topic string, content []byte) bool {
	return s.client.Publish(topic, content)
}

func getRandomString(l int) string {
	str := "0123456789abcdefghijklmnopqrstuvwxyz"
	bytes := []byte(str)
	result := []byte{}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < l; i++ {
		result = append(result, bytes[r.Intn(len(bytes))])
	}
	return string(result)
}

func refreshAuth(api *eos.API, mongoCli *mongo.Client) error {
	entry := log.WithFields(log.Fields{Function: "refreshAuth"})
	collection := mongoCli.Database(MinerTrackerDB).Collection(AuthTab)
	cur, err := collection.Find(context.Background(), bson.M{})
	if err != nil {
		entry.WithError(err).Error("traversaling auth record failed")
		return err
	}
	for cur.Next(context.Background()) {
		auth := new(Auth)
		err := cur.Decode(auth)
		if err != nil {
			entry.WithError(err).Error("decoding auth failed")
			continue
		}
		pubkey, err := getPublicKey(api, auth.Account, "active")
		if err != nil {
			entry.WithField(AccountName, auth.Account).WithError(err).Error("fetching public key failed")
			continue
		}
		if strings.HasPrefix(pubkey, "YTA") || strings.HasPrefix(pubkey, "EOS") {
			pubkey = string(pubkey[3:])
		}
		if pubkey != auth.PublicKey {
			_, err := collection.UpdateOne(context.Background(), bson.M{"_id": auth.Account}, bson.M{"$set": bson.M{"publickey": pubkey}})
			if err != nil {
				entry.WithField(AccountName, auth.Account).WithError(err).Error("update public key failed")
			}
		}

	}
	cur.Close(context.Background())
	entry.Info("refreshed Auth table")
	return nil
}

func getPublicKey(api *eos.API, accountName string, perm string) (string, error) {
	accountResp, err := api.GetAccount(eos.AN(accountName))
	if err != nil {
		return "", fmt.Errorf("get account info: %s", err)
	}
	for _, p := range accountResp.Permissions {
		if p.PermName == perm {
			return p.RequiredAuth.Keys[0].PublicKey.String(), nil
		}
	}
	return "", fmt.Errorf("no key found")
}
