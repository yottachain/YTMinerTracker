package yttracker

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

//state of miner log
const (
	NEW    = "new"
	DELETE = "delete"
)

//MinerLog log of node operation
type MinerLog struct {
	ID         int64  `json:"_id"`
	MinerID    int32  `json:"minerID"`
	FromStatus int32  `json:"fromStatus"`
	ToStatus   int32  `json:"toStatus"`
	Type       string `json:"type"`
	Timestamp  int64  `json:"timestamp"`
}

//MinerLogResp struct
type MinerLogResp struct {
	MinerLogs []*MinerLog
	More      bool
	Next      int64
}

//TrackProgress struct
type TrackProgress struct {
	ID        int32 `bson:"_id"`
	Start     int64 `bson:"start"`
	Timestamp int64 `bson:"timestamp"`
}

//GetMinerLogs find miner logs
func GetMinerLogs(httpCli *http.Client, url string, from int64, count int, skipTime int64) (*MinerLogResp, error) {
	entry := log.WithFields(log.Fields{Function: "GetMinerLogs"})
	count++
	fullURL := fmt.Sprintf("%s/sync/getMinerLogs?start=%d&count=%d", url, from, count)
	entry.Debugf("fetching miner logs by URL: %s", fullURL)
	request, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		entry.WithError(err).Errorf("create request failed: %s", fullURL)
		return nil, err
	}
	request.Header.Add("Accept-Encoding", "gzip")
	resp, err := httpCli.Do(request)
	if err != nil {
		entry.WithError(err).Errorf("get miner logs failed: %s", fullURL)
		return nil, err
	}
	defer resp.Body.Close()
	reader := io.Reader(resp.Body)
	if strings.Contains(resp.Header.Get("Content-Encoding"), "gzip") {
		gbuf, err := gzip.NewReader(reader)
		if err != nil {
			entry.WithError(err).Errorf("decompress response body: %s", fullURL)
			return nil, err
		}
		reader = io.Reader(gbuf)
		defer gbuf.Close()
	}
	response := make([]*MinerLog, 0)
	err = json.NewDecoder(reader).Decode(&response)
	if err != nil {
		entry.WithError(err).Errorf("decode miner logs failed: %s", fullURL)
		return nil, err
	}
	skipByte32 := Int32ToBytes(int32(skipTime))
	padding := []byte{0x00, 0x00, 0x00, 0x00}
	skipTime64 := BytesToInt64(append(skipByte32, padding...))
	i := 0
	for _, item := range response {
		if item.ID > skipTime64 {
			break
		}
		i++
	}
	response = response[0:i]
	next := int64(0)
	entry.Debugf("fetched %d shards rebuilt", len(response))
	if len(response) == count {
		next = response[count-1].ID
		return &MinerLogResp{MinerLogs: response[0 : count-1], More: true, Next: next}, nil
	}
	return &MinerLogResp{MinerLogs: response, More: false, Next: 0}, nil
}

//TrackingStat tracking miner logs and process
func (tracker *MinerTracker) TrackingStat(ctx context.Context) {
	entry := log.WithFields(log.Fields{Function: "TrackingStat"})
	urls := tracker.minerStat.AllSyncURLs
	snCount := len(urls)
	collectionMiner := tracker.dbCli.Database(MinerTrackerDB).Collection(NodeTab)
	collectionProgress := tracker.dbCli.Database(MinerTrackerDB).Collection(TrackProgressTab)
	for i := 0; i < snCount; i++ {
		snID := int32(i)
		go func() {
			entry.Infof("starting tracking SN%d", snID)
			for {
				record := new(TrackProgress)
				err := collectionProgress.FindOne(ctx, bson.M{"_id": snID}).Decode(record)
				if err != nil {
					if err == mongo.ErrNoDocuments {
						record = &TrackProgress{ID: snID, Start: 0, Timestamp: time.Now().Unix()}
						_, err := collectionProgress.InsertOne(ctx, record)
						if err != nil {
							entry.WithError(err).Errorf("insert tracking progress: %d", snID)
							time.Sleep(time.Duration(tracker.minerStat.WaitTime) * time.Second)
							continue
						}
					} else {
						entry.WithError(err).Errorf("finding tracking progress: %d", snID)
						time.Sleep(time.Duration(tracker.minerStat.WaitTime) * time.Second)
						continue
					}
				}
				minerLogs, err := GetMinerLogs(tracker.httpCli, tracker.minerStat.AllSyncURLs[snID], record.Start, tracker.minerStat.BatchSize, time.Now().Unix()-int64(tracker.minerStat.SkipTime))
				if err != nil {
					time.Sleep(time.Duration(tracker.minerStat.WaitTime) * time.Second)
					continue
				}
				for _, item := range minerLogs.MinerLogs {
					if item.Type == NEW && item.FromStatus == -1 {
						_, err := collectionMiner.InsertOne(ctx, bson.M{"_id": item.MinerID, "status": item.ToStatus, "regtime": item.Timestamp})
						if err != nil {
							errstr := err.Error()
							if strings.ContainsAny(errstr, "duplicate key error") {
								collectionMiner.UpdateOne(ctx, bson.M{"_id": item.MinerID}, bson.M{"$set": bson.M{"regtime": item.Timestamp}})
							} else {
								entry.WithError(err).Errorf("insert new miner %d", item.MinerID)
							}
						} else {
							entry.Infof("new miner %d has been registered", item.MinerID)
						}
					} else if item.Type == DELETE && item.ToStatus == -1 {
						_, err := collectionMiner.DeleteOne(ctx, bson.M{"_id": item.MinerID})
						if err != nil {
							entry.WithError(err).Errorf("delete miner %d", item.MinerID)
						} else {
							entry.Infof("miner %d has been deleted", item.MinerID)
						}
					}
				}
				if minerLogs.More {
					collectionProgress.UpdateOne(ctx, bson.M{"_id": snID}, bson.M{"$set": bson.M{"start": minerLogs.Next, "timestamp": time.Now().Unix()}})
				} else {
					if len(minerLogs.MinerLogs) > 0 {
						next := minerLogs.MinerLogs[len(minerLogs.MinerLogs)-1].ID + 1
						collectionProgress.UpdateOne(ctx, bson.M{"_id": snID}, bson.M{"$set": bson.M{"start": next, "timestamp": time.Now().Unix()}})
					}
					time.Sleep(time.Duration(tracker.minerStat.WaitTime) * time.Second)
				}
			}
		}()
	}
}
