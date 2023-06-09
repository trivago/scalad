package main

import (
	"time"

	log "github.com/sirupsen/logrus"
)

func fireMapTicker() {
	ticker := time.NewTicker(time.Second * time.Duration(fireMapTickerEnvInt))

	go func() {
		for _ = range ticker.C {
			checkFiringMap()
		}
	}()

}

func scalerTicker() {
	ticker := time.NewTicker(time.Second * time.Duration(scalerTickerEnvInt))

	var err error
	go func() {
		for _ = range ticker.C {
			jobMap, err = getJobs()
			if err != nil {
				log.Error("Error getting jobs from nomad from inside Ticker with err: ", err)
			}
			checkMeta(jobMap)
			prometheusQueries(jobMetaMap)
		}
	}()

}
