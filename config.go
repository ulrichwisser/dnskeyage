package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path"

	"github.com/ulrichwisser/zonestats/dnsresolver"

	yaml "gopkg.in/yaml.v2"
)

type Configuration struct {
	Dryrun       bool
	Verbose      bool
	Source       string
	Resolvers    stringslice
	Zones        stringslice
	Port         uint
	InfluxServer string
	InfluxDB     string
	InfluxUser   string
	InfluxPasswd string
}

type stringslice []string

func (str *stringslice) String() string {
	return fmt.Sprintf("%s", *str)
}

func (str *stringslice) Set(value string) error {
	*str = append(*str, value)
	return nil
}

func parseCmdline() *Configuration {
	var config Configuration
	var conffilename string

	// define and parse command line arguments
	flag.StringVar(&conffilename, "conf", "", "Filename to read configuration from")
	flag.BoolVar(&config.Dryrun, "dryrun", false, "Nothing will be written to InfluxDB")
	flag.BoolVar(&config.Verbose, "v", true, "Print lots of runtime information")
	flag.Var(&config.Zones, "zone", "zone to compute dnskey age for")
	flag.Var(&config.Resolvers, "resolver", "resolver name or ip")
	flag.StringVar(&config.InfluxServer, "influxServer", "", "Server with InfluxDB running")
	flag.StringVar(&config.InfluxDB, "influxDB", "", "Name of InfluxDB database")
	flag.StringVar(&config.InfluxUser, "influxUser", "", "Name of InfluxDB user")
	flag.StringVar(&config.InfluxPasswd, "influxPasswd", "", "Name of InfluxDB user password")
	flag.Parse()

	var confFromFile *Configuration
	if conffilename != "" {
		var err error
		confFromFile, err = readConfigFile(conffilename)
		if err != nil {
			panic(err)
		}
	}
	return joinConfig(confFromFile, &config)
}

func readConfigFile(filename string) (config *Configuration, error error) {
	source, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	config = &Configuration{}
	err = yaml.Unmarshal(source, config)
	if err != nil {
		return nil, err
	}
	return config, nil
}

func readDefaultConfigFiles() (config *Configuration) {

	// .dzone in current directory
	usr, err := user.Current()
	if err != nil {
		panic(err)
	}
	fileconfig, err := readConfigFile(path.Join(usr.HomeDir, ".dnskeyage"))
	if err != nil && !os.IsNotExist(err) {
		panic(err)
	}
	config = joinConfig(config, fileconfig)

	// .dzone in user home directory
	fileconfig, err = readConfigFile(".dnskeyage")
	if err != nil && !os.IsNotExist(err) {
		panic(err)
	}
	config = joinConfig(config, fileconfig)

	// done
	return
}

func joinConfig(oldConf *Configuration, newConf *Configuration) (config *Configuration) {
	if oldConf == nil && newConf == nil {
		return nil
	}
	if oldConf != nil && newConf == nil {
		return oldConf
	}
	if oldConf == nil && newConf != nil {
		return newConf
	}

	// we have two configs, join them
	config = &Configuration{}
	config.Dryrun = newConf.Dryrun || oldConf.Dryrun
	config.Verbose = newConf.Verbose || oldConf.Verbose
	if len(newConf.Zones) > 0 {
		config.Zones = newConf.Zones
	} else {
		config.Zones = oldConf.Zones
	}
	if len(newConf.Resolvers) > 0 {
		config.Resolvers = newConf.Resolvers
	} else {
		config.Resolvers = oldConf.Resolvers
	}
	if newConf.InfluxServer != "" {
		config.InfluxServer = newConf.InfluxServer
	} else {
		config.InfluxServer = oldConf.InfluxServer
	}
	if newConf.InfluxDB != "" {
		config.InfluxDB = newConf.InfluxDB
	} else {
		config.InfluxDB = oldConf.InfluxDB
	}
	if newConf.InfluxUser != "" {
		config.InfluxUser = newConf.InfluxUser
	} else {
		config.InfluxUser = oldConf.InfluxUser
	}
	if newConf.InfluxPasswd != "" {
		config.InfluxPasswd = newConf.InfluxPasswd
	} else {
		config.InfluxPasswd = oldConf.InfluxPasswd
	}

	// Done
	return config
}

func usage() {
	flag.Usage()
	os.Exit(1)
}

func checkConfiguration(config *Configuration) *Configuration {
	// Get resolvers to use
	if len(config.Resolvers) == 0 {
		config.Resolvers = dnsresolver.GetDefaultResolvers()
	}
	if len(config.Resolvers) == 0 {
		fmt.Println("No resolver(s) found.")
		usage()
	}

	// Port
	if config.Port == 0 {
		config.Port = 53
	}

	// Get zones to use
	if len(config.Zones) == 0 {
		fmt.Println("No zones given.")
		usage()
	}

	// Influx config
	if !config.Dryrun {
		if len(config.InfluxServer) == 0 {
			fmt.Println("Influx server address must be given.")
			usage()
		}
		if len(config.InfluxDB) == 0 {
			fmt.Println("Influx server address must be given.")
			usage()
		}
		if (len(config.InfluxUser) == 0) && (len(config.InfluxPasswd) > 0) {
			fmt.Println("Influx user and password must be given (not only one).")
			usage()
		}
		if (len(config.InfluxUser) > 0) && (len(config.InfluxPasswd) == 0) {
			fmt.Println("Influx user and password must be given (not only one).")
			usage()
		}
	}
	return config
}
