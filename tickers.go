package main

import (
	"time"

	log "github.com/Sirupsen/logrus"
)

func fireMapTicker() {
	ticker := time.NewTicker(time.Second * 30)

	go func() {
		for _ = range ticker.C {
			checkFiringMap()
		}
	}()

}

func scalerTicker() {
	ticker := time.NewTicker(time.Minute * 1)

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
