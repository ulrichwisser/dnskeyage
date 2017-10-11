package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/miekg/dns"

	influxdb "github.com/influxdata/influxdb/client/v2"
	"github.com/influxdata/influxdb/models"
)

func getInfluxClient(config *Configuration) (influxdb.Client, error) {
	// Create a new HTTPClient
	return influxdb.NewHTTPClient(influxdb.HTTPConfig{
		Addr:     config.InfluxServer,
		Username: config.InfluxUser,
		Password: config.InfluxPasswd,
	})
}

type key struct {
	timestamp int64
	domain    string
	algorithm string
	keytag    uint16
	age       int64
}

func parseRow(config *Configuration, row models.Row) (*key, error) {
	key := &key{}
	t, err := time.Parse(time.RFC3339, row.Values[0][0].(string))
	if err != nil {
		if config.Verbose {
			log.Println("Could not convert to time ", row.Values[0][0].(string), "\n", err)
		}
		return nil, err
	}
	key.timestamp = t.Unix()
	key.domain = row.Values[0][1].(string)
	key.algorithm = row.Values[0][2].(string)
	if val, err := strconv.ParseUint(row.Values[0][3].(string), 10, 16); err == nil {
		key.keytag = uint16(val)
	} else {
		return nil, err
	}
	key.age, err = row.Values[0][4].(json.Number).Int64()
	if err != nil {
		return nil, err
	}
	return key, nil
}

func parseResponse(config *Configuration, response *influxdb.Response) ([]key, error) {
	// first check for errors
	if response.Error() != nil {
		return nil, response.Error()
	}

	// prepare to collect key data
	keys := make([]key, 0)

	// loop over all rows
	for _, result := range response.Results {
		if len(result.Err) > 0 {
			log.Fatalf("Result error!\n%s", result.Err)
		}
		for _, msg := range result.Messages {
			if config.Verbose {
				log.Printf("Result message: %s %s", msg.Level, msg.Text)
			}
		}
		for _, row := range result.Series {
			key, err := parseRow(config, row)
			if err != nil {
				if config.Verbose {
					log.Println("Error in row.\n", "Row ", row, "\n", err)
				}
				return nil, err
			}
			keys = append(keys, *key)
		}
	}

	// done
	return keys, nil
}

func getOldKeys(config *Configuration, database influxdb.Client, zone string) ([]key, error) {
	q := influxdb.Query{
		Command:  "select domain,algorithm,keytag,first(age) from DnskeyAge where domain='" + zone + "' group by domain,algorithm,keytag",
		Database: config.InfluxDB,
	}
	response, err := database.Query(q)
	if err != nil {
		return nil, err
	}
	return parseResponse(config, response)
}

func getAge(oldkeys []key, zone string, algorithm string, keytag uint16) (age int64) {
	age = 0
	for _, oldkey := range oldkeys {
		if oldkey.domain == zone && oldkey.algorithm == algorithm && oldkey.keytag == keytag {
			age = time.Now().Unix() - oldkey.timestamp
			break
		}
	}
	return age
}

func saveToInflux(config *Configuration, database influxdb.Client, zone string, newkeys []*dns.DNSKEY, oldkeys []key) {

	bp, err := influxdb.NewBatchPoints(influxdb.BatchPointsConfig{
		Database:  config.InfluxDB,
		Precision: "s",
	})
	if err != nil {
		log.Fatal(err)
	}

	// collect line data
	for _, rr := range newkeys {
		// compute more human friendly formats
		var algorithm string
		if alg, ok := dns.AlgorithmToString[rr.Algorithm]; ok {
			algorithm = alg
		} else {
			algorithm = fmt.Sprintf("%d", rr.Algorithm)
		}
		keytype := fmt.Sprintf("%d", rr.Flags)
		if rr.Flags == 257 {
			keytype = "KSK"
		}
		if rr.Flags == 256 {
			keytype = "ZSK"
		}

		tags := map[string]string{
			"domain":    zone,
			"algorithm": algorithm,
			"keytag":    fmt.Sprintf("%d", rr.KeyTag()),
			"keytype":   keytype,
		}

		fields := map[string]interface{}{
			"age": getAge(oldkeys, zone, algorithm, rr.KeyTag()),
		}

		pt, err := influxdb.NewPoint(
			"DnskeyAge",
			tags,
			fields,
			time.Now(),
		)
		if err != nil {
			log.Fatal(err)
		}
		bp.AddPoint(pt)
	}

	// Verobse
	if config.Verbose {
		for _, p := range bp.Points() {
			log.Println("Point: ", p)
		}
	}

	// save to database
	if config.Dryrun {
		if config.Verbose {
			log.Println("DRYRUN! Nothing will be written to InfluxDB.")
		}
	} else {
		if err := database.Write(bp); err != nil {
			log.Fatal(err)
		}
		if config.Verbose {
			log.Println("Successfully written to InfluxDB")
		}
	}
}
