package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/trivago/scalad/slack"
	"github.com/trivago/scalad/structs"
	log "github.com/Sirupsen/logrus"
	"github.com/go-chi/chi"
	tparse "github.com/karrick/tparse/v2"
)

// Scaler jobMap handler.
type Scaler struct {
	jobMap map[string]structs.JobStruct
}

// newScaler created the jobMap and also starts the startJobMapWatcher and
// returns the map where all allocation in cooldown will be stored.
func newScaler() Scaler {
	scaler := Scaler{jobMap: make(map[string]structs.JobStruct)}
	scaler.startJobMapWatcher()
	return scaler
}

// health function is an http enpoint used for consul to check the health of the application.
// If it is healthy it will retun a: http/200 All Good message
func (scaler *Scaler) health(w http.ResponseWriter, r *http.Request) {
	message := "<html>All Good</html>"
	fmt.Fprintf(w, "%s", message)
}

func (scaler *Scaler) stopScallingJob(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobName")
	timer := chi.URLParam(r, "timer")
	mapID := jobID + "-" + region
	now := time.Now()
	var job structs.JobStruct
	sleep, err := tparse.AddDuration(now, timer)
	if err != nil {
		log.Debug("Error parsing time for pause command with err: ", err)
		return
	}
	job.ScaleCooldown = sleep
	mutex.Lock()
	scaler.jobMap[mapID] = job
	mutex.Unlock()
	message := "Manually paused: " + mapID + " for " + timer
	slack.SendMessage(message)
	fmt.Fprintf(w, "%s", message)
}

func (scaler *Scaler) resumeScallingJob(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobName")
	mapID := jobID + "-" + region

	jobMapMutex.Lock()
	jobMapScaleMutex.Lock()

	log.Debug("Refreshing job config for ", jobID)
	delete(jobMap, jobID)
	delete(jobMapScale, jobID)
	jobMapScaleMutex.Unlock()

	nomadJob, err := GetJob(jobID, region)
	if err != nil {
		log.Warn("Error getting job ", jobID, " with err: ", err)
	} else {
		jobMap[jobID] = &nomadJob
	}
	jobMapMutex.Unlock()

	mutex.Lock()
	defer mutex.Unlock()
	delete(scaler.jobMap, mapID)

	message := "Manually resumed: " + mapID
	slack.SendMessage(message)
	fmt.Fprintf(w, "%s", message)
}

func (scaler *Scaler) scaleAction(body []byte) (err error) {
	postStruct := new(structs.PostRequest)
	err = json.Unmarshal(body, postStruct)
	if err != nil {
		log.Error("Body: ", string(body))
		log.Error("Error Unmarshalling postJson with err: ", err)
		return err
	}

	for k := range postStruct.Alerts {
		allocID := postStruct.Alerts[k].Labels.AllocID
		jobID := postStruct.Alerts[k].Labels.JobName
		log.Debug("Recieved scale for: ", jobID, " with alertname: ", postStruct.Alerts[k].Labels.Alertname)

		if len(region) == 0 {
			log.Error("No region defined for AllocID: ", allocID)
			continue
		}
		status := postStruct.Alerts[k].Status
		if len(region) < 1 {
			log.Info("No region defined for Alert: ", jobID)
			continue
		}
		if len(jobID) < 1 {
			log.Info("No JobName defined for Alert")
			continue
		}
		log.Debug("Jobname recieved: ", jobID, " Region: ", region)

		if strings.HasPrefix(postStruct.Alerts[k].Labels.Alertname, "scaleup") {
			if strings.HasPrefix(status, "firing") {
				err := scaler.ScaleUp(jobID, region)
				if err != nil {
					log.Debug("Job: ", jobID, " Error: ", err)
				}
			}

			log.Debug("Status: ", status, " for ", jobID, " Region: ", region)

		} else if strings.HasPrefix(postStruct.Alerts[k].Labels.Alertname, "scaledown") {
			if strings.HasPrefix(status, "firing") {
				err := scaler.ScaleDown(jobID, region)
				if err != nil {
					log.Debug("Job: ", jobID, " Error: ", err)
				}
			}

			log.Debug("Status: ", status, " for ", jobID, " Region: ", region)

		}
	}
	return nil
}

// scale function gets a POST requests and analizes the content to decide which scale direction to apply
// or to discard the message. The POST requests comes from alertmanager.
func (scaler *Scaler) scale(w http.ResponseWriter, r *http.Request) {

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(400)
		fmt.Fprintf(w, "Error reading request Body, with err: %v", err)
		return
	}

	go scaler.scaleAction(body)

	w.WriteHeader(200)
}
