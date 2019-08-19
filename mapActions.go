package main

import (
	"time"
)

// startJopMapWatcher starts a ticker to check the cooldown expiracy every 5 seconds in the map.
func (scaler *Scaler) startJobMapWatcher() {
	ticker := time.NewTicker(time.Second * 5)

	go func() {
		for _ = range ticker.C {
			scaler.cleanMap()
		}
	}()
}

// cleanMap functions checks all the entries in the map for expired cooldonws and removed them from the map.
func (scaler *Scaler) cleanMap() {
	mutex.Lock()
	for key, job := range scaler.jobMap {
		now := time.Now()

		if now.After(job.ScaleCooldown) {
			delete(scaler.jobMap, key)

		}
	}
	mutex.Unlock()
}
