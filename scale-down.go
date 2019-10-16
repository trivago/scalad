package main

import (
	"fmt"
	"net/http"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/go-chi/chi"
	nomad "github.com/hashicorp/nomad/api"
	"github.com/trivago/scalad/job"
	"github.com/trivago/scalad/slack"
	"github.com/trivago/scalad/structs"
)

// ScaleDown function checks that the current job is not in cooldown in the map and if it is not
// checks for every group in the jobfile that needs to be scaled.
func (scaler *Scaler) ScaleDown(jobID string, region string) (err error) {
	now := time.Now()
	mapID := jobID + "-" + region
	mutex.Lock()
	_, ok := scaler.jobMap[mapID]
	mutex.Unlock()
	if ok {
		mutex.Lock()
		diff := now.Sub(scaler.jobMap[mapID].ScaleCooldownDown)
		mutex.Unlock()
		log.Info("Job: ", jobID, " ScaleDown can be retrigger in: ", diff)
		return fmt.Errorf("Job in cooldown")
	}

	var nomadJob nomad.Job
	jobMapMutex.Lock()
	_, ok = jobMap[jobID]
	if ok {
		nomadJob = *jobMap[jobID]
	} else {
		nomadJob, err = GetJob(nomadHost, jobID, region)
		if err != nil {
			log.Warn("Error getting job with err: ", err)
			return err
		}
	}
	jobMapMutex.Unlock()

	var AnyTrue bool
	groupsMap, nomadJob := job.ParseJSON(nomadJob, "down")

	for _, job := range groupsMap {
		if (job.ScaleMin <= 0) || (job.ScaleMax <= 0) || (job.ScaleCountUp <= 0) || (job.ScaleCountDown <= 0) || (job.Count <= 0) {
			log.Warn(jobID, "Group: ", job.Group, " doesn't have a scale stanza in it.")
			job.NoGo = true
		}

		job.ScaleCooldown = job.ScaleCooldownDown
		mutex.Lock()
		scaler.jobMap[mapID] = job
		mutex.Unlock()

		if job.Count <= job.ScaleMin {
			log.Info("Job: ", jobID, " Group: ", job.GroupName, " in: ", region, " is at MinCount (", job.ScaleMin, " allocations)")
			job.NoGo = true
		} else if job.Count > job.ScaleMax {
			log.Info("Job ", jobID, " Group: ", job.GroupName, " in: ", region, " is above the MaxCount")
			job.NoGo = true
		} else {
			job.NoGo = false
		}
		structLocal := groupsMap[job.GroupName]
		structLocal = job
		groupsMap[job.GroupName] = structLocal
	}

	for _, job := range groupsMap {
		if job.NoGo == false {
			log.Debug(job.GroupName, " Group needs to be scaled Down.")
			AnyTrue = true
		}
	}

	if AnyTrue {
		p := log.Debug
		p("Scaling UP: ")
		p("JobName:          ", jobID)

		for _, job := range groupsMap {
			p("Group:            ", job.GroupName)
			if job.TaskName != "" {
				p("TaskName:         ", job.TaskName)
			}
			p("Region:           ", job.Region)
			p("ScaleMin:         ", job.ScaleMin)
			p("ScaleMax:         ", job.ScaleMax)
			p("ScaleCountUp:     ", job.ScaleCountUp)
			p("ScaleCountDown:   ", job.ScaleCountDown)
			p("Count:            ", job.Count)
			p("ScaleCooldown:    ", job.ScaleCooldown)
		}
		err := ScaleJobDown(groupsMap, nomadJob)
		if err != nil {
			log.Error("Scale up failed with err: ", err)
			return err
		}
	}

	return nil
}

// ScaleJobDown calculate the new amount of allocations necesary for every group in the job and sends the request to nomad to
// scale the job. It also updates the list of the last 20 executed jobs after sending the request to nomad.
func ScaleJobDown(groupsMap map[string]structs.JobStruct, nomadJob nomad.Job) error {
	for _, job := range groupsMap {
		if job.Count == job.ScaleMin {
			job.EndValue = job.ScaleMin
			job.NoGo = true
		} else {
			job.EndValue = job.Count - job.ScaleCountDown
			if job.EndValue <= job.ScaleMin {
				job.EndValue = job.ScaleMin
				log.Info("Scaling down Job: ", job.JobName, " Group:", job.GroupName, " to minimum allowed. Min: ", job.ScaleMin)
				job.NoGo = false
			}
			log.Info("Job: "+job.JobName+" Group: "+job.GroupName+" on: "+job.Region+" NewCount is: ", job.EndValue)
		}
		structLocal := groupsMap[job.GroupName]
		structLocal.EndValue = job.EndValue
		groupsMap[job.GroupName] = structLocal
	}

	for _, newJob := range nomadJob.TaskGroups {
		if groupsMap[*newJob.Name].EndValue != 0 {
			*newJob.Count = groupsMap[*newJob.Name].EndValue
		}
		log.Info("Job: ", *nomadJob.Name, " Group: ", *newJob.Name, " NewCount: ", *newJob.Count)
	}

	ok, err := executeJob(nomadJob)
	if !ok {
		log.Error("Error executing scaledown operation!")
		return err
	}

	message := `SCALE DOWN:
	- Job: ` + *nomadJob.Name + `
	- Region: ` + *nomadJob.Region
	slack.SendMessage(message)
	slack.MessageBuffered(*nomadJob.Name, "down", time.Now())

	scalerVec.WithLabelValues(*nomadJob.Name, *nomadJob.Region, "down").Inc()
	LastJobs(*nomadJob.Name, *nomadJob.Region, "scaleDown", time.Now())
	return nil
}

func manualScaleDown(w http.ResponseWriter, r *http.Request) {
	jobName := chi.URLParam(r, "jobName")
	region := chi.URLParam(r, "region")
	user, pass, _ := r.BasicAuth()
	if user == username && pass == password {
		nomadJob, err := GetJob(nomadHost, jobName, region)
		if err != nil {
			log.Warn("Error getting job with err: ", err)
			return
		}
		for _, taskGroup := range nomadJob.TaskGroups {
			*taskGroup.Count--
			if *taskGroup.Count == 0 {
				*taskGroup.Count = 1
			}
		}

		ok, err := executeJob(nomadJob)
		if !ok {
			log.Error("Error executing manual scaleup operation!")
			fmt.Fprintf(w, "%s", "Error executing manual scaleup operation!")
			return
		}

		message := `MANUAL SCALE DOWN for ` + jobName + ` in Region: ` + region + `
	All taskGroups count have been decreased by one!!
	For safety reason not allowed to scale to 0! Min value is 1`
		slack.SendMessage(message)
		fmt.Fprintf(w, "%s", "Manual scale down triggered!")
	} else {
		fmt.Fprintf(w, "%s", "Wrong Username or password!")
	}
}
