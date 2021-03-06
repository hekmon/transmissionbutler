package main

import (
	"flag"
	"os"
	"sync"

	"github.com/hekmon/hllogger"
	"github.com/hekmon/pushover"
	"github.com/hekmon/transmissionrpc"
	systemd "github.com/iguanesolutions/go-systemd"
)

var (
	logger         *hllogger.HlLogger
	transmission   *transmissionrpc.Client
	conf           *config
	pushoverClient *pushover.Controller
	butlerRun      sync.Mutex
)

func main() {
	// Parse flags
	logLevelFlag := flag.Int("loglevel", 1, "Set loglevel: Debug(0) Info(1) Warning(2) Error(3) Fatal(4). Default Info.")
	confFile := flag.String("conf", "config.json", "Relative or absolute path to the json configuration file")
	flag.Parse()

	// Init systemd controller
	var err error
	if !systemd.IsNotifyEnabled() {
		logger.Warning("[Main] systemd not detected: systemd special features won't be available")
	}

	// Init logger
	var ll hllogger.LogLevel
	switch *logLevelFlag {
	case 0:
		ll = hllogger.Debug
	case 1:
		ll = hllogger.Info
	case 2:
		ll = hllogger.Warning
	case 3:
		ll = hllogger.Error
	case 4:
		ll = hllogger.Fatal
	default:
		ll = hllogger.Info
	}
	logger = hllogger.New(os.Stderr, &hllogger.Config{
		LogLevel:              ll,
		SystemdJournaldCompat: systemd.IsNotifyEnabled(),
	})
	logger.Output(" ")
	logger.Output(" • Transmission Butler •")
	logger.Output("      ヽ(　￣д￣)ノ")
	logger.Output(" ")

	// Load config
	logger.Info("[Main] Loading configuration")
	if conf, err = getConfig(*confFile); err != nil {
		logger.Fatalf(1, "can't load config: %v", err)
	}
	logger.Debugf("[Main] Loaded configuration:\n%+v", conf)

	// Init pushover
	pushoverClient = pushover.New(conf.Pushover.AppKey, conf.Pushover.UserKey, logger)
	defer pushoverClient.SendHighPriorityMsg("Application is stopping...", "", "main stopping")

	// Init transmission client
	transmission, err = transmissionrpc.New(conf.Server.Host, conf.Server.User, conf.Server.Password,
		&transmissionrpc.AdvancedConfig{
			HTTPS:     conf.Server.HTTPS,
			Port:      conf.Server.Port,
			UserAgent: "github.com/hekmon/transmissionbutler",
		})
	if err != nil {
		logger.Fatalf(2, "[Main] Can't initialize the transmission client: %v", err)
	}
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

	// We are ready
	if err = systemd.NotifyReady(); err != nil {
		logger.Errorf("[Main] Can't send systemd ready notification: %v", err)
	}
	pushoverClient.SendLowPriorityMsg("Application is started ヽ(　￣д￣)ノ", "", "main")

	// Wait butler's clean stop before exiting main goroutine
	mainStop.Lock()

	// Lock the butler run in case the worker is done but not a forced run issued by USR1
	butlerRun.Lock()
	logger.Info("[Main] Exiting")
}
