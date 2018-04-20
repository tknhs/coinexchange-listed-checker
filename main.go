package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
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
	LineToken             string `toml:"line_token"`
	SlackWebhookURL       string `toml:"slack_webhook_url"`
	AccessWaitTimeSeconds int    `toml:"access_wait_time_seconds"`
	NotifyWaitTimeSeconds int    `toml:"notify_wait_time_seconds"`
}

const ApplicationName = "CoinExchange.io - Listed Checker"

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

func IsListed(tickerCode string) (bool, error) {
	success := false

	resp, err := http.Get("https://www.coinexchange.io/api/v1/getcurrency?ticker_code=" + tickerCode)
	if err != nil {
		return success, err
	}
	defer resp.Body.Close()

	var getCurrency GetCurrency
	if body, err := ioutil.ReadAll(resp.Body); err != nil {
		return success, err
	} else {
		if err = json.Unmarshal(body, &getCurrency); err != nil {
			return success, err
		}
	}

	success = getCurrency.Success == "1"
	return success, err
}

func PostToLine(tickerCode string, token string) error {
	values := url.Values{}
	values.Add("message", tickerCode)

	req, err := http.NewRequest(
		"POST",
		"https://notify-api.line.me/api/notify",
		strings.NewReader(values.Encode()),
	)
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return errors.New(resp.Status)
	}
	return nil
}

func PostToSlack(message string, webhookURL string) error {
	values := url.Values{}
	values.Add("payload", "{'text':'<!channel> "+message+"'")

	req, err := http.NewRequest(
		"POST",
		webhookURL,
		strings.NewReader(values.Encode()),
	)
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return errors.New(resp.Status)
	}
	return nil
}

func main() {
	logger := LogInit()
	config, err := LoadConfig()
	if err != nil {
		logger.Error(err.Error())
	}

	symbol := strings.ToUpper(config.General.Symbol)
	lineToken := config.General.LineToken
	slackWebhookURL := config.General.SlackWebhookURL
	notifyAccessTimeSeconds := time.Duration(config.General.AccessWaitTimeSeconds)
	notifyWaitTimeSeconds := time.Duration(config.General.NotifyWaitTimeSeconds)
	message := "https://www.coinexchange.io/market/" + symbol + "/BTC"

	isListed := false
	for !isListed {
		isListed, err = IsListed(symbol)
		if err != nil {
			logger.Error(err.Error())
		}
		if !isListed {
			time.Sleep(notifyAccessTimeSeconds * time.Second)
		}
	}

	go func() {
		if err := PostToLine(message, lineToken); err != nil {
			logger.Error(err.Error())
		}
	}()

	go func(){
	if err := PostToSlack(message, slackWebhookURL); err != nil {
		logger.Error(err.Error())
	}
	}()

	for {
		if err := beeep.Notify(ApplicationName, message, ""); err != nil {
			logger.Error(err.Error())
		}
		time.Sleep(notifyWaitTimeSeconds * time.Second)
	}
}
