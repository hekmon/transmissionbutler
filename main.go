package main

import (
	"flag"
	"os"
	"sync"

	"github.com/gregdel/pushover"
	"github.com/hekmon/hllogger"
	"github.com/hekmon/transmissionrpc"
)

var logger *hllogger.HlLogger
var transmission *transmissionrpc.Client
var conf *config
var pushoverApp *pushover.Pushover
var pushoverDest *pushover.Recipient
var butlerRun sync.Mutex

func main() {
	// Parse flags
	logLevelFlag := flag.Int("loglevel", 1, "Set loglevel: Debug(0) Info(1) Warning(2) Error(3) Fatal(4). Default Info.")
	confFile := flag.String("conf", "config.json", "Relative or absolute path to the json configuration file")
	flag.Parse()
	// Init logger
	switch *logLevelFlag {
	case 0:
		logger = hllogger.New(os.Stdout, "", hllogger.Debug, 0)
	case 1:
		logger = hllogger.New(os.Stdout, "", hllogger.Info, 0)
	case 2:
		logger = hllogger.New(os.Stdout, "", hllogger.Warning, 0)
	case 3:
		logger = hllogger.New(os.Stdout, "", hllogger.Error, 0)
	case 4:
		logger = hllogger.New(os.Stdout, "", hllogger.Fatal, 0)
	default:
		logger = hllogger.New(os.Stdout, "", hllogger.Info, 0)
	}
	logger.Output(" ")
	logger.Output(" • Transmission Butler •")
	logger.Output("      ヽ(　￣д￣)ノ")
	logger.Output(" ")
	// Load config
	var err error
	logger.Info("[Main] Loading configuration")
	if conf, err = getConfig(*confFile); err != nil {
		logger.Fatalf(1, "can't load config: %v", err)
	}
	logger.Debugf("[Main] Loaded configuration:\n%+v", conf)
	// Init pushover if enabled
	if conf.isPushoverEnabled() {
		pushoverApp = pushover.New(*conf.Pushover.AppKey)
		pushoverDest = pushover.NewRecipient(*conf.Pushover.UserKey)
	}
	// Init transmission client
	transmission = transmissionrpc.New(conf.Server.Host, conf.Server.User, conf.Server.Password,
		&transmissionrpc.AdvancedConfig{
			HTTPS:     conf.Server.HTTPS,
			Port:      conf.Server.Port,
			UserAgent: "github.com/hekmon/transmissionbutler",
		})
	ok, serverVersion, serverMinimumVersion, err := transmission.RPCVersion()
	if err != nil {
		logger.Errorf("[Main] Can't check remote transmission RPC version: %v", err)
	} else {
		if ok {
			logger.Infof("[Main] Remote transmission RPC version (v%d) is compatible with our transmissionrpc library (v%d)",
				serverVersion, transmissionrpc.RPCVersion)
		} else {
			logger.Fatalf(2, "[Main] Remote transmission RPC version (v%d) is incompatible with the transmission library (v%d): remote needs at least v%d",
				serverVersion, transmissionrpc.RPCVersion, serverMinimumVersion)
		}
	}
	// Start butler
	stopSignal := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	logger.Info("[Main] Starting butler")
	go butler(stopSignal, &wg)
	// Handles system signals properly
	var mainStop sync.Mutex
	mainStop.Lock()
	logger.Debug("[Main] Starting signal handling goroutine")
	go handleSignals(stopSignal, &wg, &mainStop)
	// Wait butler's clean stop before exiting main goroutine
	mainStop.Lock()
	// Lock the butler run in case the worker is done but not a forced run issued by USR1
	butlerRun.Lock()
	logger.Info("[Main] Exiting")
}
