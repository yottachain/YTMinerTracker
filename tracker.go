package yttracker

import (
	"context"

	"github.com/aurawing/eos-go"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

//MinerTracker miner tracker
type MinerTracker struct {
	dbCli  *mongo.Client
	Params *MiscConfig
}

//New create a new miner tracker instance
func New(mongoDBURL, eosURL string, mqconf *AuraMQConfig, miscconf *MiscConfig) (*MinerTracker, error) {
	entry := log.WithFields(log.Fields{Function: "New"})
	dbClient, err := mongo.Connect(context.Background(), options.Client().ApplyURI(mongoDBURL))
	if err != nil {
		entry.WithError(err).Errorf("creating mongo DB client failed: %s", mongoDBURL)
		return nil, err
	}
	eosAPI := eos.New(eosURL)
	_, err = StartSync(eosAPI, dbClient, mqconf.ServerConfig, mqconf.ClientConfig, miscconf)
	if err != nil {
		entry.WithError(err).Error("creating mq service failed")
		return nil, err
	}
	return &MinerTracker{dbCli: dbClient, Params: miscconf}, nil
}
