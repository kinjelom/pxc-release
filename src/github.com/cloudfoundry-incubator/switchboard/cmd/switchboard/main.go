package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"

	"github.com/cloudfoundry-incubator/cf-lager"
	"github.com/pivotal-cf-experimental/switchboard/api"
	"github.com/pivotal-cf-experimental/switchboard/config"
	"github.com/pivotal-cf-experimental/switchboard/domain"
	"github.com/pivotal-cf-experimental/switchboard/proxy"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"

	"github.com/pivotal-golang/lager"
)

func main() {
	flags := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	configFile := flags.String("configFile", "", "Path to config file")
	pidFile := flags.String("pidFile", "", "Path to pid file")
	cf_lager.AddFlags(flags)
	flags.Parse(os.Args[1:])

	logger := cf_lager.New("Switchboard")

	proxyConfig, err := config.Load(*configFile)
	if err != nil {
		logger.Fatal("Error loading config file:", err, lager.Data{"config": *configFile})
	}

	err = ioutil.WriteFile(*pidFile, []byte(strconv.Itoa(os.Getpid())), 0644)
	if err == nil {
		logger.Info(fmt.Sprintf("Wrote pidFile to %s", *pidFile))
	} else {
		logger.Fatal("Cannot write pid to file", err, lager.Data{"pidFile": *pidFile})
	}

	backends := domain.NewBackends(proxyConfig.Backends, logger)
	cluster := domain.NewCluster(
		backends,
		proxyConfig.HealthcheckTimeout(),
		logger,
	)

	group := grouper.NewParallel(os.Kill, grouper.Members{
		grouper.Member{"proxy", proxy.NewRunner(cluster, proxyConfig.Port, logger)},
		grouper.Member{"api", api.NewRunner(proxyConfig.APIPort, backends, logger)},
	})
	process := ifrit.Invoke(group)

	logger.Info(fmt.Sprintf("Proxy started with configuration: %+v\n", proxyConfig))

	err = <-process.Wait()
	if err != nil {
		logger.Fatal("Error starting switchboard", err, lager.Data{"proxyConfig": proxyConfig})
	}
}
