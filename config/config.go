package config

import (
	"fmt"
	"strconv"

	"github.com/lbryio/dispendium/actions"
	"github.com/lbryio/dispendium/env"
	"github.com/lbryio/dispendium/jobs"
	"github.com/lbryio/dispendium/util"
	"github.com/lbryio/dispendium/wallets"

	"github.com/lbryio/lbry.go/v2/lbrycrd"

	"github.com/fsnotify/fsnotify"
	"github.com/johntdyer/slackrus"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// ConfigPath is the path to the config file
var ConfigPath string

// InitializeConfiguration inits the base configuration of dispendium
func InitializeConfiguration() {
	conf, err := env.NewWithEnvVars()
	if err != nil {
		logrus.Panic(err)
	}
	if viper.GetBool("debugmode") {
		util.Debugging = true
		logrus.SetLevel(logrus.DebugLevel)
	}
	if viper.GetBool("tracemode") {
		util.Debugging = true
		logrus.SetLevel(logrus.TraceLevel)
	}
	util.AuthToken = conf.AuthToken
	readConfig()
	viper.WatchConfig()
	viper.OnConfigChange(func(in fsnotify.Event) {
		fmt.Println("Config file changed:", in.Name)
		readConfig()
	})
	initSlack(conf)
	initWallets(conf)
	SetLBCConfig(conf)
}

//SetLBCConfig sets the configuration for any environment variables related to the spending of LBC
func SetLBCConfig(config *env.Config) {
	var err error
	actions.MaxLBCPerHour, err = strconv.ParseFloat(config.MaxLBCPerHour, 64)
	if err != nil {
		logrus.Panic(err)
	}
	actions.MaxLBCPayment, err = strconv.ParseFloat(config.MaxLBCPayment, 64)
	if err != nil {
		logrus.Panic(err)
	}
	jobs.MinLBCBalance, err = strconv.ParseFloat(config.MinBalance, 64)
	if err != nil {
		logrus.Panic(err)
	}
}

// initSlack initializes the slack connection and posts info level or greater to the set channel.
func initSlack(config *env.Config) {
	slackURL := config.SlackHookURL
	slackChannel := config.SlackChannel
	if slackURL != "" && slackChannel != "" {
		logrus.AddHook(&slackrus.SlackrusHook{
			HookURL:        slackURL,
			AcceptedLevels: slackrus.LevelThreshold(logrus.InfoLevel),
			Channel:        slackChannel,
			IconEmoji:      ":money_mouth_face:",
			Username:       "Dispendium",
		})

		jobs.CreditBalanceLogger = logrus.New()
		jobs.CreditBalanceLogger.AddHook(&slackrus.SlackrusHook{
			HookURL:        slackURL,
			AcceptedLevels: slackrus.LevelThreshold(logrus.InfoLevel),
			Channel:        "credit-alerts",
			IconEmoji:      ":money_mouth_face:",
			Username:       "wallet-watcher",
		})
	}
}

func initWallets(conf *env.Config) {
	chainParams, ok := lbrycrd.ChainParamsMap[conf.BlockchainName]
	if !ok {
		logrus.Panicf("block chain name %s is not recognized", conf.BlockchainName)
	}
	wallets.SetChainParams(&chainParams)
	instances := viper.GetStringMapString("lbrycrd")
	if len(instances) == 0 {
		logrus.Panic("No lbrycrd instances found in config to connect to")
	}
	for name, url := range instances {
		lbrycrdClient, err := lbrycrd.New(url, &chainParams)
		if err != nil {
			panic(err)
		}
		_, err = lbrycrdClient.GetBalance("*")
		if err != nil {
			logrus.Errorf("Error connecting to lbrycrd: %+v", err)
			continue
		}
		wallets.AddWallet(name, lbrycrdClient)
	}
}

func readConfig() {
	viper.SetConfigName("dispendium")                // name of config file (without extension)
	viper.AddConfigPath(viper.GetString(ConfigPath)) // 1 - commandline config path
	viper.AddConfigPath("$HOME/")                    // 2 - check $HOME
	viper.AddConfigPath(".")                         // 3 - optionally look for config in the working directory
	viper.AddConfigPath("./config/default/")         // 4 - use default that comes with the branch
	err := viper.ReadInConfig()                      // Find and read the config file
	if err != nil {                                  // Handle errors reading the config file
		logrus.Warning("Error reading config file...defaults will be used: ", err)
	}
}
