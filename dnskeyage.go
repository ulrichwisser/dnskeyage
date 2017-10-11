package main

import (
	"log"

	"github.com/miekg/dns"
)

func main() {
	// get configuration
	config := joinConfig(readDefaultConfigFiles(), parseCmdline())
	checkConfiguration(config)

	// get dabase handle
	dbh, err := getInfluxClient(config)
	if err != nil {
		log.Fatal(err)
	}

	// check all zones
	for _, zone := range config.Zones {
		if config.Verbose {
			log.Printf("Run for zone %s", zone)
		}
		newkeys := getNewKeys(config, dns.Fqdn(zone))
		if len(newkeys) == 0 {
			if config.Verbose {
				log.Printf("No keys found for %s", zone)
			}
			continue
		}
		if config.Verbose {
			log.Printf("%d keys found for zone %s", len(newkeys), zone)
		}
		oldkeys, err := getOldKeys(config, dbh, zone)
		if err != nil {
			log.Println("Error in getoldkeys!", err)
			continue
		}
		saveToInflux(config, dbh, zone, newkeys, oldkeys)
	}

	// done
}
