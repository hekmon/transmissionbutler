package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

func getConfig(filename string) (conf *config, err error) {
	// Open file
	var configFile *os.File
	if configFile, err = os.Open(filename); err != nil {
		err = fmt.Errorf("can't open '%s' for reading: %v", filename, err)
		return
	}
	defer configFile.Close()
	// Parse it
	if err = json.NewDecoder(configFile).Decode(&conf); err != nil {
		err = fmt.Errorf("can't decode '%s' as JSON: %v", filename, err)
		return
	}
	// Check values
	if conf.Server.Host == "" {
		err = fmt.Errorf("server host can't be empty")
		return
	}
	if conf.Server.Port == 0 {
		err = fmt.Errorf("server port can't be 0")
		return
	}
	if conf.Butler.CheckFrequency == 0 {
		err = fmt.Errorf("butler check frequency can't be 0")
		return
	}
	if conf.Butler.TargetRatio <= 0 {
		err = fmt.Errorf("target ratio lesser than or equals to 0 make no sense")
		return
	}
	// All good
	return
}

type config struct {
	Server   serverConfig   `json:"server"`
	Butler   butlerConfig   `json:"butler"`
	Pushover pushoverConfig `json:"pushover"`
}

func (c *config) isPushoverEnabled() bool {
	return c.Pushover.AppKey != nil && c.Pushover.UserKey != nil
}

type serverConfig struct {
	Host     string `json:"host"`
	Port     uint16 `json:"port"`
	HTTPS    bool   `json:"https"`
	User     string `json:"user"`
	Password string `json:"password"`
}

type butlerConfig struct {
	CheckFrequency time.Duration `json:"check_frequency_minutes"`
	FreeSeed       time.Duration `json:"free_seed_days"`
	TargetRatio    float64       `json:"target_ratio"`
	DeleteDone     bool          `json:"delete_when_done"`
}

type pushoverConfig struct {
	AppKey  *string `json:"app_key"`
	UserKey *string `json:"user_key"`
}

func (bc *butlerConfig) UnmarshalJSON(data []byte) (err error) {
	type rawButlerConfig butlerConfig
	tmp := &struct {
		*rawButlerConfig
	}{
		rawButlerConfig: (*rawButlerConfig)(bc),
	}
	if err = json.Unmarshal(data, tmp); err == nil {
		bc.CheckFrequency *= time.Minute
		bc.FreeSeed *= 24 * time.Hour
	}
	return
}
