package main

import (
	"context"
	"fmt"
	"os"
	"time"

	yt "github.com/yottachain/yotta-miner-tracker"
	"github.com/yottachain/yotta-miner-tracker/cmd"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	cmd.Execute()
}

func main1() {
	fmt.Printf("start time: %s\n", time.Now().String())
	var mongoURL string = "mongodb://127.0.0.1:27017/?connect=direct"
	var snID int = -1
	var err error
	if len(os.Args) > 1 {
		mongoURL = os.Args[1]
	}
	mongoCli, err := mongo.Connect(context.Background(), options.Client().ApplyURI(mongoURL))
	if err != nil {
		panic(fmt.Sprintf("creating mongo DB client failed: %s\n", mongoURL))
	}
	shardsTab := mongoCli.Database("metabase").Collection("shards")
	nodeTab := mongoCli.Database("yotta").Collection("Node")
	seqTab := mongoCli.Database("yotta").Collection("Sequence")
	checkPointTab := mongoCli.Database("test").Collection("CheckPoint")
	endPointTab := mongoCli.Database("test").Collection("EndPoint")
	calcNodeTab := mongoCli.Database("test").Collection("CalcNode")
	tempNodeTab := mongoCli.Database("test").Collection("TempNode")
	sequence := new(Sequence)
	err = seqTab.FindOne(context.Background(), bson.M{"_id": 101}).Decode(sequence)
	if err != nil {
		fmt.Println("read SN ID failed")
		panic(err)
	}
	snID = sequence.Seq
	fmt.Printf("current SN ID: %d\n", snID)
	fmt.Println("======== 1. read or create endpoint and temp nodes collection ========")
	end := new(EndPoint)
	err = endPointTab.FindOne(context.Background(), bson.M{"_id": 1}).Decode(end)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			_, err := tempNodeTab.DeleteMany(context.Background(), bson.M{})
			if err != nil {
				fmt.Println("delete all temp nodes failed")
				panic(err)
			}
			fmt.Println("delete all temp nodes")
			cur, err := nodeTab.Find(context.Background(), bson.M{})
			if err != nil {
				fmt.Println("read node collection failed")
				panic(err)
			}
			for cur.Next(context.Background()) {
				node := new(yt.Node)
				err := cur.Decode(node)
				if err != nil {
					panic(err)
				}
				tempNode := &TempNode{ID: node.ID, Space: node.Uspaces[fmt.Sprintf("sn%d", snID)]}
				_, err = tempNodeTab.InsertOne(context.Background(), tempNode)
				if err != nil {
					fmt.Printf("create temp node failed, ID: %d\n", node.ID)
					panic(err)
				}
			}
			cur.Close(context.Background())
			end.ID = 1
			end.End = time.Now().Unix() << 32
			_, err = endPointTab.InsertOne(context.Background(), end)
			if err != nil {
				fmt.Println("insert endpoint failed")
				panic(err)
			}
			fmt.Printf("insert endpoint: %d\n", end.End)
			_, err = nodeTab.UpdateMany(context.Background(), bson.M{}, bson.M{"$set": bson.M{"uspaces.del": 0}})
			if err != nil {
				fmt.Println("clear uspaces.del field failed")
				panic(err)
			} else {
				fmt.Println("clear uspaces.del field of all nodes")
			}
		} else {
			fmt.Println("read endpoint failed")
			panic(err)
		}
	} else {
		fmt.Printf("read endpoint: %d\n", end.End)
	}
	fmt.Println("======== 2. read or create checkpoint ========")
	check := new(CheckPoint)
	err = checkPointTab.FindOne(context.Background(), bson.M{"_id": 1}).Decode(check)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			check.ID = 1
			check.CheckPoint = 0
			_, err = checkPointTab.InsertOne(context.Background(), check)
			if err != nil {
				fmt.Println("insert checkpoint failed")
				panic(err)
			}
			fmt.Printf("insert checkpoint: %d\n", check.CheckPoint)
		} else {
			fmt.Println("read checkpoint failed")
			panic(err)
		}
	} else {
		fmt.Printf("read checkpoint: %d\n", check.CheckPoint)
	}
	fmt.Println("======== 3. calculate shards count ========")
	total := 0
	var limit int64 = 500000
	opt := options.FindOptions{}
	opt.Sort = bson.M{"_id": 1}
	opt.Limit = &limit
	for {
		var lastID int64 = 0
		count := 0
		cache := make(map[int32]int64)
		cur, err := shardsTab.Find(context.Background(), bson.M{"_id": bson.M{"$gte": check.CheckPoint}}, &opt)
		if err != nil {
			fmt.Println("read shards collection failed")
			panic(err)
		}
		for cur.Next(context.Background()) {
			count++
			shard := new(Shard)
			err := cur.Decode(shard)
			if err != nil {
				fmt.Println("decode shard failed")
				panic(err)
			}
			if shard.NodeID != 0 {
				cache[shard.NodeID] += 1
			}
			if shard.NodeID != shard.NodeID2 && shard.NodeID2 != 0 {
				cache[shard.NodeID2] += 1
			}
			lastID = shard.ID
		}
		cur.Close(context.Background())
		if count == 0 {
			break
		}
		total += count
		fmt.Printf("read %d shards\n", total)
		for k, v := range cache {
			res := calcNodeTab.FindOneAndUpdate(context.Background(), bson.M{"_id": k, "check": bson.M{"$lte": check.CheckPoint}}, bson.M{"$set": bson.M{"check": lastID + 1}, "$inc": bson.M{"space": v}})
			if res.Err() != nil {
				if res.Err() == mongo.ErrNoDocuments {
					_, err := calcNodeTab.InsertOne(context.Background(), bson.M{"_id": k, "space": v, "check": lastID + 1})
					if err != nil {
						fmt.Printf("insert calc node %d failed: %d\n", k, v)
					}
				} else {
					fmt.Printf("update count failed: %d->%d\n", k, v)
					panic(err)
				}
			}
		}
		_, err = checkPointTab.UpdateOne(context.Background(), bson.M{"_id": 1}, bson.M{"$set": bson.M{"checkpoint": lastID + 1}})
		if err != nil {
			fmt.Printf("update checkpoint failed: %d\n", lastID+1)
			panic(err)
		} else {
			check.CheckPoint = lastID + 1
			fmt.Printf("update checkpoint: %d\n", lastID+1)
		}
	}
	fmt.Println("======== 4. correct shards count of node ========")
	cur, err := calcNodeTab.Find(context.Background(), bson.M{})
	if err != nil {
		fmt.Println("read calcnode collection failed")
		panic(err)
	}
	for cur.Next(context.Background()) {
		calcnode := new(CalcNode)
		err := cur.Decode(calcnode)
		if err != nil {
			panic(err)
		}
		tempnode := new(TempNode)
		err = tempNodeTab.FindOne(context.Background(), bson.M{"_id": calcnode.ID}).Decode(tempnode)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				fmt.Printf("cannot find temp %d\n", calcnode.ID)
				continue
			} else {
				fmt.Printf("read temp node error: %d\n", calcnode.ID)
				panic(err)
			}
		}
		res := nodeTab.FindOneAndUpdate(context.Background(), bson.M{"_id": calcnode.ID, "correct": bson.M{"$exists": false}}, bson.M{"$set": bson.M{"correct": true}, "$inc": bson.M{fmt.Sprintf("uspaces.sn%d", snID): calcnode.Space - tempnode.Space}})
		if res.Err() != nil {
			if res.Err() == mongo.ErrNoDocuments {
				fmt.Printf("skip updating node %d when update uspace\n", calcnode.ID)
				continue
			} else {
				fmt.Printf("update uspace of node %d failed\n", calcnode.ID)
				panic(err)
			}
		}
		fmt.Printf("update uspace of node %d: %d\n", calcnode.ID, calcnode.Space-tempnode.Space)
	}
	cur.Close(context.Background())
	_, err = nodeTab.UpdateMany(context.Background(), bson.M{}, bson.M{"$unset": bson.M{"correct": true}})
	if err != nil {
		fmt.Println("remove correct field failed")
		panic(err)
	} else {
		fmt.Println("remove correct field of nodes")
	}
	fmt.Printf("end time: %s\n", time.Now().String())
}

type Shard struct {
	ID      int64 `bson:"_id"`
	NodeID  int32 `bson:"nodeId"`
	NodeID2 int32 `bson:"nodeId2"`
}

type TempNode struct {
	ID    int32 `bson:"_id"`
	Space int64 `bson:"space"`
}

type CalcNode struct {
	ID    int32 `bson:"_id"`
	Space int64 `bson:"space"`
	Check int64 `bson:"check"`
}

type CheckPoint struct {
	ID         int32 `bson:"_id"`
	CheckPoint int64 `bson:"checkpoint"`
}

type EndPoint struct {
	ID  int32 `bson:"_id"`
	End int64 `bson:"end"`
}

type Sequence struct {
	ID  int32 `bson:"_id"`
	Seq int   `bson:"seq"`
}
