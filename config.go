package yttracker

//Field names of config file
const (
	//bind address of http server
	HTTPBindAddrField = "http-bind-addr"
	//URL of EOS service
	EOSURLField = "eos-url"
	//URL of mongo DB
	MongoDBURLField = "mongodb-url"

	//config of MQ
	AuramqServerBindAddrField             = "auramq.server.bind-addr"
	AuramqServerRouterBufferSizeField     = "auramq.server.router-buffer-size"
	AuramqServerSubscriberBufferSizeField = "auramq.server.subscriber-buffer-size"
	AuramqServerReadBufferSizeField       = "auramq.server.read-buffer-size"
	AuramqServerWriteBufferSizeField      = "auramq.server.write-buffer-size"
	AuramqServerPingWaitField             = "auramq.server.ping-wait"
	AuramqServerReadWaitField             = "auramq.server.read-wait"
	AuramqServerWriteWaitField            = "auramq.server.write-wait"
	AuramqServerMinerSyncTopicField       = "auramq.server.miner-sync-topic"

	AuramqClientSubscriberBufferSizeField = "auramq.client.subscriber-buffer-size"
	AuramqClientPingWaitField             = "auramq.client.ping-wait"
	AuramqClientReadWaitField             = "auramq.client.read-wait"
	AuramqClientWriteWaitField            = "auramq.client.write-wait"
	AuramqClientMinerSyncTopicField       = "auramq.client.miner-sync-topic"
	AuramqClientAllSNURLsField            = "auramq.client.all-sn-urls"
	AuramqClientAccountField              = "auramq.client.account"
	AuramqClientPrivateKeyField           = "auramq.client.private-key"
	AuramqClientClientIDField             = "auramq.client.client-id"

	//MinerStat config
	MinerStatAllSyncURLsField = "miner-stat.all-sync-urls"
	MinerStatBatchSizeField   = "miner-stat.batch-size"
	MinerStatWaitTimeField    = "miner-stat.wait-time"
	MinerStatSkipTimeField    = "miner-stat.skip-time"

	//Log config
	LoggerOutputField       = "logger.output"
	LoggerFilePathField     = "logger.file-path"
	LoggerRotationTimeField = "logger.rotation-time"
	LoggerMaxAgeField       = "logger.max-age"
	LoggerLevelField        = "logger.level"

	//Misc config
	MiscRefreshAuthIntervalField = "misc.refresh-auth-interval"
)

//Config system configuration
type Config struct {
	HTTPBindAddr string           `mapstructure:"http-bind-addr"`
	EOSURL       string           `mapstructure:"eos-url"`
	MongoDBURL   string           `mapstructure:"mongodb-url"`
	AuraMQ       *AuraMQConfig    `mapstructure:"auramq"`
	MinerStat    *MinerStatConfig `mapstructure:"miner-stat"`
	Logger       *LogConfig       `mapstructure:"logger"`
	Misc         *MiscConfig      `mapstructure:"misc"`
}

//AuraMQConfig auramq configuration
type AuraMQConfig struct {
	ServerConfig *ServerConfig `mapstructure:"server"`
	ClientConfig *ClientConfig `mapstructure:"client"`
}

//ServerConfig server config of AuraMQ
type ServerConfig struct {
	BindAddr             string `mapstructure:"bind-addr"`
	RouterBufferSize     int    `mapstructure:"router-buffer-size"`
	SubscriberBufferSize int    `mapstructure:"subscriber-buffer-size"`
	ReadBufferSize       int    `mapstructure:"read-buffer-size"`
	WriteBufferSize      int    `mapstructure:"write-buffer-size"`
	PingWait             int    `mapstructure:"ping-wait"`
	ReadWait             int    `mapstructure:"read-wait"`
	WriteWait            int    `mapstructure:"write-wait"`
	MinerSyncTopic       string `mapstructure:"miner-sync-topic"`
}

//ClientConfig client config of AuraMQ
type ClientConfig struct {
	SubscriberBufferSize int      `mapstructure:"subscriber-buffer-size"`
	PingWait             int      `mapstructure:"ping-wait"`
	ReadWait             int      `mapstructure:"read-wait"`
	WriteWait            int      `mapstructure:"write-wait"`
	MinerSyncTopic       string   `mapstructure:"miner-sync-topic"`
	AllSNURLs            []string `mapstructure:"all-sn-urls"`
	Account              string   `mapstructure:"account"`
	PrivateKey           string   `mapstructure:"private-key"`
	ClientID             string   `mapstructure:"client-id"`
}

//MinerStatConfig miner log sync configuration
type MinerStatConfig struct {
	AllSyncURLs []string `mapstructure:"all-sync-urls"`
	BatchSize   int      `mapstructure:"batch-size"`
	WaitTime    int      `mapstructure:"wait-time"`
	SkipTime    int      `mapstructure:"skip-time"`
}

//LogConfig system log configuration
type LogConfig struct {
	Output       string `mapstructure:"output"`
	FilePath     string `mapstructure:"file-path"`
	RotationTime int64  `mapstructure:"rotation-time"`
	MaxAge       int64  `mapstructure:"max-age"`
	Level        string `mapstructure:"level"`
}

//MiscConfig miscellaneous configuration
type MiscConfig struct {
	RefreshAuthInterval int `mapstructure:"refresh-auth-interval"`
}
