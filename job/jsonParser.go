package job

import (
	"strconv"
	"time"

	"github.com/trivago/scalad/structs"
	log "github.com/sirupsen/logrus"
	nomad "github.com/hashicorp/nomad/api"
	tparse "github.com/karrick/tparse/v2"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	namespace                       = "scalers"
	subsystem                       = ""
	stableStopedScaleLabels         = []string{}
	stableStopedScaleEventStatusVec = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: prometheus.BuildFQName(namespace, subsystem, "stableStopedScaleEventStatus"),
			Help: "Scaling jobs",
		},
		stableStopedScaleLabels,
	)
)

func init() {
	prometheus.MustRegister(stableStopedScaleEventStatusVec)
}

// ParseJSON takes a byte array from getJob and a string and checks that this nomad Job has the
// scalling stanza correctly declared inside of it. It also checks before scalling down that the current job is stable.
// This status check is ignored for scalling up just in case the application is not stable because it is overloaded by requests.
// Returns jobs []JobStruct and nomadJob nomad.Job
func ParseJSON(orgJob nomad.Job, call string) (groupsMap map[string]structs.JobStruct, nomadJob nomad.Job) {

	groupsMap = make(map[string]structs.JobStruct)
	// Do not check if the job is stable on scalling up just in case the application is overloaded and can not get stable.
	//	if call != "up" {
	if *orgJob.Stable == false {
		log.Debug("Job: ", *orgJob.Name, " is not stable for Scalling!! Aborting Scale operation until the job get stable...")
		var labels prometheus.Labels
		if len(stableStopedScaleLabels) > 0 {
			if len(stableStopedScaleLabels) == 3 {
				labels["connector"] = stableStopedScaleLabels[0]
				labels["region"] = stableStopedScaleLabels[1]
				labels["action"] = stableStopedScaleLabels[2]
				stableStopedScaleEventStatusVec.With(labels).Inc()
			}
		}
		stableStopedScaleEventStatusVec.WithLabelValues().Inc()

		return
	}

	var err error

	//checkGroups for meta stanza and if found put it on the map
	for _, taskGroup := range orgJob.TaskGroups {
		var jsonJob structs.JobStruct
		jsonJob.GroupName = *taskGroup.Name

		jsonJob.ScaleMin, err = strconv.Atoi(taskGroup.Meta["scale_min"])
		if err != nil {
			log.Debug("Unable to convert ScaleMin to int in Task: ", *taskGroup.Name, " in job: ", *orgJob.Name)
			continue
		}
		jsonJob.ScaleMax, err = strconv.Atoi(taskGroup.Meta["scale_max"])
		if err != nil {
			log.Debug("Unable to convert ScaleMax to int in Task: ", *taskGroup.Name, " in job: ", *orgJob.Name)
			continue
		}
		jsonJob.ScaleCountUp, err = strconv.Atoi(taskGroup.Meta["scale_count_up"])
		if err != nil {
			log.Debug("Unable to convert ScaleCountUp to int in Task: ", *taskGroup.Name, " in job: ", *orgJob.Name)
			continue
		}
		jsonJob.ScaleCountDown, err = strconv.Atoi(taskGroup.Meta["scale_count_down"])
		if err != nil {
			log.Debug("Unable to convert ScaleCountDown to int in Task: ", *taskGroup.Name, " in job: ", *orgJob.Name)
			continue
		}
		jsonJob.Count = *taskGroup.Count
		jsonJob.LastRun = time.Now()
		now := time.Now()
		_, ok := taskGroup.Meta["scale_cooldown_up"]
		if ok {
			up, err := tparse.AddDuration(now, "+"+taskGroup.Meta["scale_cooldown_up"])
			if err != nil {
				log.Debug("Meta ScaleCooldownUP error: ", err)
				continue
			}
			jsonJob.ScaleCooldownUp = up
		}
		_, ok = taskGroup.Meta["scale_cooldown_down"]
		if ok {
			down, err := tparse.AddDuration(now, "+"+taskGroup.Meta["scale_cooldown_down"])
			if err != nil {
				log.Debug("Meta ScaleCooldownDown error: ", err)
				continue
			}
			jsonJob.ScaleCooldownDown = down
		}

		jsonJob.JobName = *orgJob.Name
		jsonJob.Region = *orgJob.Region
		if jsonJob.ScaleMin != 0 {
			log.Info("Adding ", jsonJob.GroupName, " to map.")
			groupsMap[jsonJob.GroupName] = jsonJob
		}
	}

	//checkGroups Tasks for meta stanza and if found put it on the map
	for i, taskGroup := range orgJob.TaskGroups {

		var jsonJob structs.JobStruct
		jsonJob.GroupName = *taskGroup.Name
		jsonJob.Count = *taskGroup.Count

		for _, tasks := range taskGroup.Tasks {
			_, exists := groupsMap[*taskGroup.Name]
			if exists {
				log.Debug("Group: ", *taskGroup.Name, " exists in group map")
				break
			}

			jsonJob.TaskName = tasks.Name
			jsonJob.ScaleMin, err = strconv.Atoi(tasks.Meta["scale_min"])
			if err != nil {
				log.Debug("Unable to convert ScaleMin to int in Taskgroup: ", *taskGroup.Name, " Task: ", tasks.Name, " in job: ", *orgJob.Name)
				continue
			}
			jsonJob.ScaleMax, err = strconv.Atoi(tasks.Meta["scale_max"])
			if err != nil {
				log.Debug("Unable to convert ScaleMax to int in Taskgroup: ", *taskGroup.Name, " Task: ", tasks.Name, " in job: ", *orgJob.Name)
				continue
			}
			jsonJob.ScaleCountUp, err = strconv.Atoi(tasks.Meta["scale_count_up"])
			if err != nil {
				log.Debug("Unable to convert ScaleCountUp to int in Taskgroup: ", *taskGroup.Name, " Task: ", tasks.Name, " in job: ", *orgJob.Name)
				continue
			}
			jsonJob.ScaleCountDown, err = strconv.Atoi(tasks.Meta["scale_count_down"])
			if err != nil {
				log.Debug("Unable to convert ScaleCountDown to int in Taskgroup: ", *taskGroup.Name, " Task: ", tasks.Name, " in job: ", *orgJob.Name)
				continue
			}
			jsonJob.LastRun = time.Now()
			now := time.Now()
			_, ok := tasks.Meta["scale_cooldown_up"]
			if ok {
				up, err := tparse.AddDuration(now, "+"+tasks.Meta["scale_cooldown_up"])
				if err != nil {
					log.Debug("Meta ScaleCooldownUP error: ", err)
					continue
				}
				jsonJob.ScaleCooldownUp = up
			}
			_, ok = tasks.Meta["scale_cooldown_down"]
			if ok {
				down, err := tparse.AddDuration(now, "+"+tasks.Meta["scale_cooldown_down"])
				if err != nil {
					log.Debug("Meta ScaleCooldownDown error: ", err)
					continue
				}
				jsonJob.ScaleCooldownDown = down
			}

			jsonJob.JobName = *orgJob.Name
			jsonJob.Region = *orgJob.Region
			jsonJob.Group = i

			groupsMap[jsonJob.GroupName] = jsonJob

		}
	}

	log.Debug("Current Map: ")
	for _, entry := range groupsMap {
		log.Debug("JobName: ", entry.JobName)
		log.Debug("	GroupName: ", entry.GroupName)
		log.Debug("	Count: ", entry.Count)
		log.Debug("	Scale Min: ", entry.ScaleMin)
		log.Debug("	Scale Max: ", entry.ScaleMax)

	}

	return groupsMap, orgJob
}
