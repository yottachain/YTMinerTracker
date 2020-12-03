package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	homedir "github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	yttracker "github.com/yottachain/yotta-miner-tracker"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "yotta-miner-tracker",
	Short: "service for publishing miner information",
	Long:  `yotta-miner-tracker is a service performing miner information publishing.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: func(cmd *cobra.Command, args []string) {
		config := new(yttracker.Config)
		if err := viper.Unmarshal(config); err != nil {
			panic(fmt.Sprintf("unable to decode into config struct, %v\n", err))
		}
		initLog(config)
		tracker, err := yttracker.New(config.MongoDBURL, config.EOSURL, config.AuraMQ, config.MinerStat, config.Misc)
		if err != nil {
			panic(fmt.Sprintf("fatal error when starting service: %s\n", err))
		}
		tracker.TrackingStat(context.Background())
		tracker.Start(config.HTTPBindAddr)
	},
}

func initLog(config *yttracker.Config) {
	switch strings.ToLower(config.Logger.Output) {
	case "file":
		writer, _ := rotatelogs.New(
			config.Logger.FilePath+".%Y%m%d",
			rotatelogs.WithLinkName(config.Logger.FilePath),
			rotatelogs.WithMaxAge(time.Duration(config.Logger.MaxAge)*time.Hour),
			rotatelogs.WithRotationTime(time.Duration(config.Logger.RotationTime)*time.Hour),
		)
		log.SetOutput(writer)
	case "stdout":
		log.SetOutput(os.Stdout)
	default:
		fmt.Printf("no such option: %s, use stdout\n", config.Logger.Output)
		log.SetOutput(os.Stdout)
	}
	log.SetFormatter(&log.TextFormatter{})
	levelMap := make(map[string]log.Level)
	levelMap["panic"] = log.PanicLevel
	levelMap["fatal"] = log.FatalLevel
	levelMap["error"] = log.ErrorLevel
	levelMap["warn"] = log.WarnLevel
	levelMap["info"] = log.InfoLevel
	levelMap["debug"] = log.DebugLevel
	levelMap["trace"] = log.TraceLevel
	log.SetLevel(levelMap[strings.ToLower(config.Logger.Level)])
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/yotta-miner-tracker.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	// rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	initFlag()
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".yotta-rebuilder" (without extension).
		viper.AddConfigPath(home)
		viper.AddConfigPath(".")
		viper.SetConfigName("yotta-miner-tracker")
		viper.SetConfigType("yaml")
	}

	// viper.AutomaticEnv() // read in environment variables that match
	// viper.SetEnvPrefix("analysis")
	// viper.SetEnvKeyReplacer(strings.NewReplacer("_", "."))

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	} else {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found; ignore error if desired
			fmt.Println("Config file not found.")
		} else {
			// Config file was found but another error was produced
			fmt.Println("Error:", err.Error())
			os.Exit(1)
		}
	}
}

var (
	//DefaultHTTPBindAddr default binding address of HTTP server
	DefaultHTTPBindAddr string = ":8080"
	//DefaultEOSURL default address of EOS server
	DefaultEOSURL string = "127.0.0.1:8888"
	//DefaultMongoDBURL default value of mongo DB URL
	DefaultMongoDBURL string = "mongodb://127.0.0.1:27017/?connect=direct"

	//DefaultAuramqServerBindAddr default binding address of AuraMQ server
	DefaultAuramqServerBindAddr string = ":8787"
	//DefaultAuramqServerRouterBufferSize default value of server side router buffer size
	DefaultAuramqServerRouterBufferSize int = 8192
	//DefaultAuramqServerSubscriberBufferSize default value of server side subscriber buffer size
	DefaultAuramqServerSubscriberBufferSize int = 1024
	//DefaultAuramqServerReadBufferSize default value of server side read buffer size
	DefaultAuramqServerReadBufferSize int = 4096
	//DefaultAuramqServerWriteBufferSize default value of server side write buffer size
	DefaultAuramqServerWriteBufferSize int = 4096
	//DefaultAuramqServerPingWait default value of server side ping wait
	DefaultAuramqServerPingWait int = 30
	//DefaultAuramqServerReadWait default value of server side read wait
	DefaultAuramqServerReadWait int = 60
	//DefaultAuramqServerWriteWait default value of server side write wait
	DefaultAuramqServerWriteWait int = 10
	//DefaultAuramqServerMinerSyncTopic default value of server side miner-sync topic
	DefaultAuramqServerMinerSyncTopic = "sync"
	//DefaultAuramqClientSubscriberBufferSize default value of client side subscriber buffer size
	DefaultAuramqClientSubscriberBufferSize int = 1024
	//DefaultAuramqClientPingWait default value of client side ping wait
	DefaultAuramqClientPingWait = 30
	//DefaultAuramqClientReadWait default value of client side read wait
	DefaultAuramqClientReadWait = 60
	//DefaultAuramqClientWriteWait default value of client side write wait
	DefaultAuramqClientWriteWait = 10
	//DefaultAuramqClientMinerSyncTopic default value of client side miner-sync topic
	DefaultAuramqClientMinerSyncTopic = "sync"
	//DefaultAuramqClientAllSNURLs default value of all MQ URLs for connecting
	DefaultAuramqClientAllSNURLs = []string{}
	//DefaultAuramqClientAccount default value of account name for authenticating
	DefaultAuramqClientAccount = ""
	//DefaultAuramqClientPrivateKey default value of private key for authenticating
	DefaultAuramqClientPrivateKey = ""
	//DefaultAuramqClientClientID default value of client ID for identifying MQ client
	DefaultAuramqClientClientID = "yottaminertracker"

	//DefaultMinerStatAllSyncURLs default value of MinerStatAllSyncURLs
	DefaultMinerStatAllSyncURLs = []string{}
	//DefaultMinerStatBatchSize default value of MinerStatBatchSize
	DefaultMinerStatBatchSize = 100
	//DefaultMinerStatWaitTime default value of MinerStatWaitTime
	DefaultMinerStatWaitTime = 10
	//DefaultMinerStatSkipTime default value of MinerStatSkipTime
	DefaultMinerStatSkipTime = 180

	//DefaultLoggerOutput default value of LoggerOutput
	DefaultLoggerOutput string = "stdout"
	//DefaultLoggerFilePath default value of LoggerFilePath
	DefaultLoggerFilePath string = "./tracker.log"
	//DefaultLoggerRotationTime default value of LoggerRotationTime
	DefaultLoggerRotationTime int64 = 24
	//DefaultLoggerMaxAge default value of LoggerMaxAge
	DefaultLoggerMaxAge int64 = 240
	//DefaultLoggerLevel default value of LoggerLevel
	DefaultLoggerLevel string = "Info"

	//DefaultMiscRefreshAuthInterval default value of auth table refreshing interval
	DefaultMiscRefreshAuthInterval int = 600
)

func initFlag() {
	//main config
	rootCmd.PersistentFlags().String(yttracker.HTTPBindAddrField, DefaultHTTPBindAddr, "Binding address of HTTP server")
	viper.BindPFlag(yttracker.HTTPBindAddrField, rootCmd.PersistentFlags().Lookup(yttracker.HTTPBindAddrField))
	rootCmd.PersistentFlags().String(yttracker.EOSURLField, DefaultEOSURL, "URL of EOS server")
	viper.BindPFlag(yttracker.EOSURLField, rootCmd.PersistentFlags().Lookup(yttracker.EOSURLField))
	rootCmd.PersistentFlags().String(yttracker.MongoDBURLField, DefaultMongoDBURL, "URL of mongoDB")
	viper.BindPFlag(yttracker.MongoDBURLField, rootCmd.PersistentFlags().Lookup(yttracker.MongoDBURLField))
	//AuraMQ config
	rootCmd.PersistentFlags().String(yttracker.AuramqServerBindAddrField, DefaultAuramqServerBindAddr, "binding address of AuraMQ server")
	viper.BindPFlag(yttracker.AuramqServerBindAddrField, rootCmd.PersistentFlags().Lookup(yttracker.AuramqServerBindAddrField))
	rootCmd.PersistentFlags().Int(yttracker.AuramqServerRouterBufferSizeField, DefaultAuramqServerRouterBufferSize, "server side router buffer size")
	viper.BindPFlag(yttracker.AuramqServerRouterBufferSizeField, rootCmd.PersistentFlags().Lookup(yttracker.AuramqServerRouterBufferSizeField))
	rootCmd.PersistentFlags().Int(yttracker.AuramqServerSubscriberBufferSizeField, DefaultAuramqServerSubscriberBufferSize, "server side subscriber buffer size")
	viper.BindPFlag(yttracker.AuramqServerSubscriberBufferSizeField, rootCmd.PersistentFlags().Lookup(yttracker.AuramqServerSubscriberBufferSizeField))
	rootCmd.PersistentFlags().Int(yttracker.AuramqServerReadBufferSizeField, DefaultAuramqServerReadBufferSize, "server side read buffer size")
	viper.BindPFlag(yttracker.AuramqServerReadBufferSizeField, rootCmd.PersistentFlags().Lookup(yttracker.AuramqServerReadBufferSizeField))
	rootCmd.PersistentFlags().Int(yttracker.AuramqServerWriteBufferSizeField, DefaultAuramqServerWriteBufferSize, "server side write buffer size")
	viper.BindPFlag(yttracker.AuramqServerWriteBufferSizeField, rootCmd.PersistentFlags().Lookup(yttracker.AuramqServerWriteBufferSizeField))
	rootCmd.PersistentFlags().Int(yttracker.AuramqServerPingWaitField, DefaultAuramqServerPingWait, "server side ping wait time")
	viper.BindPFlag(yttracker.AuramqServerPingWaitField, rootCmd.PersistentFlags().Lookup(yttracker.AuramqServerPingWaitField))
	rootCmd.PersistentFlags().Int(yttracker.AuramqServerReadWaitField, DefaultAuramqServerReadWait, "server side read wait time")
	viper.BindPFlag(yttracker.AuramqServerReadWaitField, rootCmd.PersistentFlags().Lookup(yttracker.AuramqServerReadWaitField))
	rootCmd.PersistentFlags().Int(yttracker.AuramqServerWriteWaitField, DefaultAuramqServerWriteWait, "server side write wait time")
	viper.BindPFlag(yttracker.AuramqServerWriteWaitField, rootCmd.PersistentFlags().Lookup(yttracker.AuramqServerWriteWaitField))
	rootCmd.PersistentFlags().String(yttracker.AuramqServerMinerSyncTopicField, DefaultAuramqServerMinerSyncTopic, "server side miner-sync topic name")
	viper.BindPFlag(yttracker.AuramqServerMinerSyncTopicField, rootCmd.PersistentFlags().Lookup(yttracker.AuramqServerMinerSyncTopicField))
	rootCmd.PersistentFlags().Int(yttracker.AuramqClientSubscriberBufferSizeField, DefaultAuramqClientSubscriberBufferSize, "client side subscriber buffer size")
	viper.BindPFlag(yttracker.AuramqClientSubscriberBufferSizeField, rootCmd.PersistentFlags().Lookup(yttracker.AuramqClientSubscriberBufferSizeField))
	rootCmd.PersistentFlags().Int(yttracker.AuramqClientPingWaitField, DefaultAuramqClientPingWait, "client side ping wait time")
	viper.BindPFlag(yttracker.AuramqClientPingWaitField, rootCmd.PersistentFlags().Lookup(yttracker.AuramqClientPingWaitField))
	rootCmd.PersistentFlags().Int(yttracker.AuramqClientReadWaitField, DefaultAuramqClientReadWait, "client side read wait time")
	viper.BindPFlag(yttracker.AuramqClientReadWaitField, rootCmd.PersistentFlags().Lookup(yttracker.AuramqClientReadWaitField))
	rootCmd.PersistentFlags().Int(yttracker.AuramqClientWriteWaitField, DefaultAuramqClientWriteWait, "client side write wait time")
	viper.BindPFlag(yttracker.AuramqClientWriteWaitField, rootCmd.PersistentFlags().Lookup(yttracker.AuramqClientWriteWaitField))
	rootCmd.PersistentFlags().String(yttracker.AuramqClientMinerSyncTopicField, DefaultAuramqClientMinerSyncTopic, "client side miner-sync topic name")
	viper.BindPFlag(yttracker.AuramqClientMinerSyncTopicField, rootCmd.PersistentFlags().Lookup(yttracker.AuramqClientMinerSyncTopicField))
	rootCmd.PersistentFlags().StringSlice(yttracker.AuramqClientAllSNURLsField, DefaultAuramqClientAllSNURLs, "all AuraMQ URLs for connecting, in the form of --auramq.all-sn-urls \"URL1,URL2,URL3\"")
	viper.BindPFlag(yttracker.AuramqClientAllSNURLsField, rootCmd.PersistentFlags().Lookup(yttracker.AuramqClientAllSNURLsField))
	rootCmd.PersistentFlags().String(yttracker.AuramqClientAccountField, DefaultAuramqClientAccount, "BP account for authenticating")
	viper.BindPFlag(yttracker.AuramqClientAccountField, rootCmd.PersistentFlags().Lookup(yttracker.AuramqClientAccountField))
	rootCmd.PersistentFlags().String(yttracker.AuramqClientPrivateKeyField, DefaultAuramqClientPrivateKey, "private key of account for authenticating")
	viper.BindPFlag(yttracker.AuramqClientPrivateKeyField, rootCmd.PersistentFlags().Lookup(yttracker.AuramqClientPrivateKeyField))
	rootCmd.PersistentFlags().String(yttracker.AuramqClientClientIDField, DefaultAuramqClientClientID, "client ID for identifying MQ client")
	viper.BindPFlag(yttracker.AuramqClientClientIDField, rootCmd.PersistentFlags().Lookup(yttracker.AuramqClientClientIDField))
	//MinerStat config
	rootCmd.PersistentFlags().StringSlice(yttracker.MinerStatAllSyncURLsField, DefaultMinerStatAllSyncURLs, "all URLs of sync services, in the form of --miner-stat.all-sync-urls \"URL1,URL2,URL3\"")
	viper.BindPFlag(yttracker.MinerStatAllSyncURLsField, rootCmd.PersistentFlags().Lookup(yttracker.MinerStatAllSyncURLsField))
	rootCmd.PersistentFlags().Int(yttracker.MinerStatBatchSizeField, DefaultMinerStatBatchSize, "batch size when fetching miner logs")
	viper.BindPFlag(yttracker.MinerStatBatchSizeField, rootCmd.PersistentFlags().Lookup(yttracker.MinerStatBatchSizeField))
	rootCmd.PersistentFlags().Int(yttracker.MinerStatWaitTimeField, DefaultMinerStatWaitTime, "wait time when no new miner logs can be fetched")
	viper.BindPFlag(yttracker.MinerStatWaitTimeField, rootCmd.PersistentFlags().Lookup(yttracker.MinerStatWaitTimeField))
	rootCmd.PersistentFlags().Int(yttracker.MinerStatSkipTimeField, DefaultMinerStatSkipTime, "ensure not to fetching miner logs till the end")
	viper.BindPFlag(yttracker.MinerStatSkipTimeField, rootCmd.PersistentFlags().Lookup(yttracker.MinerStatSkipTimeField))
	//logger config
	rootCmd.PersistentFlags().String(yttracker.LoggerOutputField, DefaultLoggerOutput, "Output type of logger(stdout or file)")
	viper.BindPFlag(yttracker.LoggerOutputField, rootCmd.PersistentFlags().Lookup(yttracker.LoggerOutputField))
	rootCmd.PersistentFlags().String(yttracker.LoggerFilePathField, DefaultLoggerFilePath, "Output path of log file")
	viper.BindPFlag(yttracker.LoggerFilePathField, rootCmd.PersistentFlags().Lookup(yttracker.LoggerFilePathField))
	rootCmd.PersistentFlags().Int64(yttracker.LoggerRotationTimeField, DefaultLoggerRotationTime, "Rotation time(hour) of log file")
	viper.BindPFlag(yttracker.LoggerRotationTimeField, rootCmd.PersistentFlags().Lookup(yttracker.LoggerRotationTimeField))
	rootCmd.PersistentFlags().Int64(yttracker.LoggerMaxAgeField, DefaultLoggerMaxAge, "Within the time(hour) of this value each log file will be kept")
	viper.BindPFlag(yttracker.LoggerMaxAgeField, rootCmd.PersistentFlags().Lookup(yttracker.LoggerMaxAgeField))
	rootCmd.PersistentFlags().String(yttracker.LoggerLevelField, DefaultLoggerLevel, "Log level(Trace, Debug, Info, Warning, Error, Fatal, Panic)")
	viper.BindPFlag(yttracker.LoggerLevelField, rootCmd.PersistentFlags().Lookup(yttracker.LoggerLevelField))
	//Misc config
	rootCmd.PersistentFlags().Int(yttracker.MiscRefreshAuthIntervalField, DefaultMiscRefreshAuthInterval, "auth table refreshing interval")
	viper.BindPFlag(yttracker.MiscRefreshAuthIntervalField, rootCmd.PersistentFlags().Lookup(yttracker.MiscRefreshAuthIntervalField))
}
