package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/Sirupsen/logrus"
	"github.com/gen2brain/beeep"
	"github.com/mitchellh/osext"
)

type GetCurrency struct {
	Success string `json:"success"`
}

type Config struct {
	General GeneralConfig `toml:"General"`
}
type GeneralConfig struct {
	Symbol                string `toml:"symbol"`
	AccessWaitTimeSeconds int    `toml:"access_wait_time_seconds"`
	NotifyWaitTimeSeconds int    `toml:"notify_wait_time_seconds"`
}

const ApplicationName = "CoinExchange.io - Listed Checker"

var Logger = LogInit()

func LogInit() *logrus.Logger {
	logger := logrus.New()
	logger.Formatter = new(logrus.JSONFormatter)

	binPath, _ := osext.ExecutableFolder()
	logPath := binPath + "error.log"

	f, _ := os.OpenFile(logPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	logger.Out = f

	return logger
}

func LoadConfig() (*Config, error) {
	binPath, _ := osext.ExecutableFolder()
	tomlPath := binPath + "config.toml"

	c := &Config{}
	_, err := toml.DecodeFile(tomlPath, &c)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func IsListed(tickerCode string) (isListed bool) {
	isListed = false

	resp, err := http.Get("https://www.coinexchange.io/api/v1/getcurrency?ticker_code=" + tickerCode)
	if err != nil {
		Logger.Error(err.Error())
		return
	}
	defer resp.Body.Close()

	var getCurrency GetCurrency
	if body, err := ioutil.ReadAll(resp.Body); err != nil {
		Logger.Error(err.Error())
		return
	} else {
		if err = json.Unmarshal(body, &getCurrency); err != nil {
			Logger.Error(err.Error())
			return
		}
	}

	isListed = getCurrency.Success == "1"
	return
}

func main() {
	config, err := LoadConfig()
	if err != nil {
		Logger.Error(err.Error())
	}

	symbol := config.General.Symbol
	notifyAccessTimeSeconds := time.Duration(config.General.AccessWaitTimeSeconds)
	notifyWaitTimeSeconds := time.Duration(config.General.NotifyWaitTimeSeconds)

	isListed := false
	for !isListed {
		isListed = IsListed(symbol)
		if !isListed {
			time.Sleep(notifyAccessTimeSeconds * time.Second)
		}
	}

	for {
		if err := beeep.Notify(ApplicationName, "Listed: "+symbol, ""); err != nil {
			Logger.Error(err.Error())
		}
		time.Sleep(notifyWaitTimeSeconds * time.Second)
	}
}
