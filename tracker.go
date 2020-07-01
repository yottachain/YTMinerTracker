package yttracker

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/aurawing/eos-go"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	log "github.com/sirupsen/logrus"
	"github.com/tylerb/graceful"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

//MinerTracker miner tracker
type MinerTracker struct {
	server *echo.Echo
	dbCli  *mongo.Client
	params *MiscConfig
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
	server := echo.New()
	return &MinerTracker{server: server, dbCli: dbClient, params: miscconf}, nil
}

//Start HTTP server
func (tracker *MinerTracker) Start(bindAddr string) error {
	entry := log.WithFields(log.Fields{Function: "Start"})
	tracker.server.Use(middleware.Logger())
	tracker.server.Use(middleware.Recover())
	tracker.server.Use(middleware.GzipWithConfig(middleware.GzipConfig{
		Level: 5,
	}))
	tracker.server.POST("/query", tracker.QueryHandler)
	tracker.server.Server.Addr = bindAddr
	err := graceful.ListenAndServe(tracker.server.Server, 5*time.Second)
	if err != nil {
		entry.WithError(err).Error("start tracker service failed")
	}
	return err
}

//FilterMiners find miners by condition
func (tracker *MinerTracker) FilterMiners(q bson.M) ([]*Node, error) {
	collection := tracker.dbCli.Database(MinerTrackerDB).Collection(NodeTab)
	cur, err := collection.Find(context.Background(), q)
	if err != nil {
		return nil, err
	}
	defer cur.Close(context.Background())
	nodes := make([]*Node, 0)
	for cur.Next(context.Background()) {
		node := new(Node)
		err := cur.Decode(node)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

//QueryHandler process miner info query
func (tracker *MinerTracker) QueryHandler(c echo.Context) error {
	entry := log.WithFields(log.Fields{Function: "QueryHandler"})
	var reader = io.Reader(c.Request().Body)
	if strings.Contains(c.Request().Header.Get("Content-Encoding"), "gzip") {
		gbuf, err := gzip.NewReader(reader)
		if err != nil {
			entry.WithError(err).Errorf("decompress request body")
			return c.String(http.StatusInternalServerError, err.Error())
		}
		reader = io.Reader(gbuf)
		defer gbuf.Close()
	}
	lines, err := ioutil.ReadAll(reader)
	if err != nil {
		tracker.server.Logger.Errorf("error when reading request body: %s\n", err.Error())
		return c.String(http.StatusInternalServerError, err.Error())
	}
	entry.Debugf("query: %s", lines)
	q := bson.M{}
	if len(lines) != 0 {
		err = json.Unmarshal(lines, &q)
		if err != nil {
			tracker.server.Logger.Errorf("error when unmarshaling query condition: %s\n", err.Error())
			return c.String(http.StatusInternalServerError, err.Error())
		}
	}
	nodes, err := tracker.FilterMiners(q)
	if err != nil {
		entry.WithError(err).Errorf("filter nodes: %+v", q)
		return c.String(http.StatusInternalServerError, err.Error())
	}
	b, err := json.Marshal(nodes)
	if err != nil {
		entry.WithError(err).Error("marshaling miner information to json")
		return c.String(http.StatusInternalServerError, err.Error())
	}
	return c.JSONBlob(http.StatusOK, b)
}
