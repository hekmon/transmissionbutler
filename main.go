package main

import (
	"flag"
	"os"
	"sync"

	"github.com/gregdel/pushover"
	"github.com/hekmon/hllogger"
	"github.com/hekmon/transmissionrpc"
	systemd "github.com/iguanesolutions/go-systemd"
)

var (
	logger       *hllogger.HlLogger
	transmission *transmissionrpc.Client
	conf         *config
	sysd         *systemd.Notifier
	pushoverApp  *pushover.Pushover
	pushoverDest *pushover.Recipient
	butlerRun    sync.Mutex
)

func main() {
	// Parse flags
	logLevelFlag := flag.Int("loglevel", 1, "Set loglevel: Debug(0) Info(1) Warning(2) Error(3) Fatal(4). Default Info.")
	confFile := flag.String("conf", "config.json", "Relative or absolute path to the json configuration file")
	flag.Parse()

	// Init systemd controller
	var err error
	if sysd, err = systemd.NewNotifier(); err != nil {
		logger.Warningf("[Main] can't start systemd notifier, systemd functions won't be enabled: %v\n", err)
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
		SystemdJournaldCompat: sysd != nil,
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

	// Init pushover if enabled
	if conf.isPushoverEnabled() {
		pushoverApp = pushover.New(*conf.Pushover.AppKey)
		pushoverDest = pushover.NewRecipient(*conf.Pushover.UserKey)
		if logger.IsDebugShown() {
			msg := "Application is starting... ヽ(　￣д￣)ノ"
			if answer, err := pushoverApp.SendMessage(pushover.NewMessage(msg), pushoverDest); err == nil {
				logger.Debugf("[Main] Successfully sent the debug message to pushover: %s", answer)
			} else {
				logger.Errorf("[Main] Can't send debug msg to pushover: %v", err)
			}
		}
	}

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

	// Wait butler's clean stop before exiting main goroutine
	mainStop.Lock()

	// Lock the butler run in case the worker is done but not a forced run issued by USR1
	butlerRun.Lock()
	logger.Info("[Main] Exiting")
}
