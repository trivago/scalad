package main

import (
	"net/http"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/alecthomas/template"
)

type lastJob struct {
	JobID     string
	Region    string
	Direction string
	Time      time.Time
}

// LastJobs function updates the lastJobs map with the jobs executed in the last X (INFO_TIME env variable) minutes.
func LastJobs(jobID string, region string, direction string, trigerTime time.Time) {
	secs := trigerTime.Unix()
	var m lastJob

	m.JobID = jobID
	m.Region = region
	m.Direction = direction
	m.Time = trigerTime

	lastJobs[secs] = m
}

func clearInfoMap() {
	now := time.Now().Unix()
	infoTimeInt64, err := strconv.ParseInt(infoTime, 10, 64)
	if err != nil {
		log.Error("Error converting int to int64 with err: ", err, ". Setting infoTimeInt64 to 60 minutes")
		infoTimeInt64 = 60
	}
	for key := range lastJobs {
		if (now - (infoTimeInt64 * 60)) > key {
			delete(lastJobs, key)
		}
	}
}

// StatusPage function returns an html page displaying the last 20 scalling operations performed.
func StatusPage(w http.ResponseWriter, r *http.Request) {
	message, err := Asset("templates/info.html")
	if err != nil {
		log.Error("Error loading asset for info.html with err: ", err)
		return
	}

	messageTmpl, err := template.New("message").Parse(string(message))
	if err != nil {
		log.Error("Error rendering template for info.html with err: ", err)
		return
	}

	info := struct {
		LastJobs map[int64]lastJob
		InfoTime string
	}{
		lastJobs,
		infoTime,
	}

	messageTmpl.Execute(w, info)

}
