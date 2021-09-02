package yttracker

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
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
	server    *echo.Echo
	dbCli     *mongo.Client
	httpCli   *http.Client
	minerStat *MinerStatConfig
	params    *MiscConfig
}

//New create a new miner tracker instance
func New(mongoDBURL, eosURL string, mqconf *AuraMQConfig, msConfig *MinerStatConfig, miscconf *MiscConfig) (*MinerTracker, error) {
	entry := log.WithFields(log.Fields{Function: "New"})
	dbClient, err := mongo.Connect(context.Background(), options.Client().ApplyURI(mongoDBURL))
	if err != nil {
		entry.WithError(err).Errorf("creating mongo DB client failed: %s", mongoDBURL)
		return nil, err
	}
	entry.Infof("mongoDB connected: %s", mongoDBURL)
	eosAPI := eos.New(eosURL)
	entry.Infof("EOS server connected: %s", eosURL)
	_, err = StartSync(eosAPI, dbClient, mqconf.ServerConfig, mqconf.ClientConfig, miscconf)
	if err != nil {
		entry.WithError(err).Error("creating MQ service failed")
		return nil, err
	}
	entry.Info("sync service started")
	server := echo.New()
	return &MinerTracker{server: server, dbCli: dbClient, httpCli: &http.Client{}, minerStat: msConfig, params: miscconf}, nil
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
	tracker.server.POST("/stablestat/reset", tracker.ResetHandler)
	tracker.server.POST("/stablestat/refresh", tracker.RefreshHandler)
	tracker.server.Server.Addr = bindAddr
	err := graceful.ListenAndServe(tracker.server.Server, 5*time.Second)
	if err != nil {
		entry.WithError(err).Error("start tracker service failed")
	}
	return err
}

//RefreshHandler refresh ratio of stable statictics
func (tracker *MinerTracker) RefreshHandler(c echo.Context) error {
	entry := log.WithFields(log.Fields{Function: "RefreshHandler"})
	cond := bson.M{}
	idstr := c.QueryParam("id")
	if idstr != "" {
		id, err := strconv.Atoi(idstr)
		if err != nil {
			entry.WithError(err).Errorf("invalid param %s", idstr)
			return c.String(http.StatusInternalServerError, err.Error())
		}
		cond = bson.M{"_id": id}
	}
	collection := tracker.dbCli.Database(MinerTrackerDB).Collection(NodeTab)
	cur, err := collection.Find(context.Background(), cond)
	if err != nil {
		entry.WithError(err).Error("find miner info for refreshing")
		return c.String(http.StatusInternalServerError, err.Error())
	}
	defer cur.Close(context.Background())
	for cur.Next(context.Background()) {
		node := new(Node)
		err := cur.Decode(node)
		if err != nil {
			entry.WithError(err).Error("decoding miner info")
			continue
		}
		ratio := float32(node.StableStat.Counter*60) / float32(time.Now().Unix()-node.StableStat.StartTime)
		if ratio > 1 {
			ratio = 1
		}
		_, err = collection.UpdateOne(context.Background(), bson.M{"_id": node.ID}, bson.M{"$set": bson.M{"stableStat.ratio": ratio}})
		if err != nil {
			entry.WithError(err).Error("update ratio of miner %d", node.ID)
			continue
		}
	}
	return c.String(http.StatusOK, "success")
}

//ResetHandler reset stable statistics
func (tracker *MinerTracker) ResetHandler(c echo.Context) error {
	entry := log.WithFields(log.Fields{Function: "ResetHandler"})
	cond := bson.M{}
	idstr := c.QueryParam("id")
	if idstr != "" {
		id, err := strconv.Atoi(idstr)
		if err != nil {
			entry.WithError(err).Errorf("invalid param %s", idstr)
			return c.String(http.StatusInternalServerError, err.Error())
		}
		cond = bson.M{"_id": id}
	}

	now := time.Now().Unix()
	entry.Infof("reset start time of stable statistics to %d", now)
	collection := tracker.dbCli.Database(MinerTrackerDB).Collection(NodeTab)
	_, err := collection.UpdateMany(context.Background(), cond, bson.M{"$set": bson.M{"stableStat.startTime": now, "stableStat.counter": 0}})
	if err != nil {
		entry.WithError(err).Errorf("reset start time of stable statistics to %d", now)
		return c.String(http.StatusInternalServerError, err.Error())
	}
	return c.String(http.StatusOK, "success")
}

//FilterMiners find miners by condition
func (tracker *MinerTracker) FilterMiners(q bson.M, sortparam string, ascparam bool, limitparam int64) ([]*Node, error) {
	collection := tracker.dbCli.Database(MinerTrackerDB).Collection(NodeTab)
	opt := new(options.FindOptions)
	asc := 1
	if !ascparam {
		asc = -1
	}
	if sortparam != "" {
		opt.Sort = bson.M{sortparam: asc}
	}
	if limitparam != 0 {
		opt.Limit = &limitparam
	}
	cur, err := collection.Find(context.Background(), q, opt)
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
	entry.Debugf("executing query: %s", lines)
	q := bson.M{}
	if len(lines) != 0 {
		err = json.Unmarshal(lines, &q)
		if err != nil {
			tracker.server.Logger.Errorf("error when unmarshaling query condition: %s\n", err.Error())
			return c.String(http.StatusInternalServerError, err.Error())
		}
	}
	sortstr := c.QueryParam("sort")
	ascstr := c.QueryParam("asc")
	asc := true
	if ascstr != "" {
		asc, err = strconv.ParseBool(ascstr)
		if err != nil {
			entry.WithError(err).Errorf("invalid asc param %s", ascstr)
			return c.String(http.StatusInternalServerError, err.Error())
		}
	}
	limitstr := c.QueryParam("limit")
	limit := 0
	if limitstr != "" {
		limit, err = strconv.Atoi(limitstr)
		if err != nil {
			entry.WithError(err).Errorf("invalid limit param %s", limitstr)
			return c.String(http.StatusInternalServerError, err.Error())
		}
	}
	nodes, err := tracker.FilterMiners(q, sortstr, asc, int64(limit))
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
