package structs

import (
	"time"

	nomad "github.com/hashicorp/nomad/api"
)

// JobStruct is where the meta data extracted from each nomad job is keept.
type JobStruct struct {
	JobName           string
	Region            string
	ScaleMin          int
	ScaleMax          int
	ScaleCountUp      int
	ScaleCountDown    int
	ScaleCooldown     time.Time
	ScaleCooldownUp   time.Time
	ScaleCooldownDown time.Time
	LastRun           time.Time
	Count             int
	Group             int
	NoGo              bool
	EndValue          int
	GroupName         string
	TaskName          string
}

// PostR is the post response that is sent to nomad to trigger the scalling action.
type PostR struct {
	Job nomad.Job
}

// PostRequest is the struct where the alert coming from alert manager would be stored.
type PostRequest struct {
	Receiver string `json:"receiver"`
	Status   string `json:"status"`
	Alerts   []struct {
		Status string `json:"status"`
		Labels struct {
			Alertname string `json:"alertname"`
			Region    string `json:"region"`
			Client    string `json:"client"`
			Instance  string `json:"instance"`
			Job       string `json:"job"`
			JobName   string `json:"exported_job"`
			AllocID   string `json:"alloc_id"`
			Monitor   string `json:"monitor"`
			Rt        string `json:"rt"`
			Severity  string `json:"severity"`
		} `json:"labels"`
		Annotations struct {
			Description string `json:"description"`
			Summary     string `json:"summary"`
		} `json:"annotations"`
		StartsAt     time.Time `json:"startsAt"`
		EndsAt       time.Time `json:"endsAt"`
		GeneratorURL string    `json:"generatorURL"`
	} `json:"alerts"`
	GroupLabels struct {
		Alertname string `json:"alertname"`
	} `json:"groupLabels"`
	CommonLabels struct {
		Alertname string `json:"alertname"`
		Job       string `json:"job"`
		Monitor   string `json:"monitor"`
		Severity  string `json:"severity"`
	} `json:"commonLabels"`
	CommonAnnotations struct {
	} `json:"commonAnnotations"`
	ExternalURL string `json:"externalURL"`
	Version     string `json:"version"`
	Time        time.Time
}

// AllocRequest is the structure that matches the response for an allocation query to nomad
// in order to get the JobName and JobRegion
type AllocRequest struct {
	Job struct {
		Region string `json:"Region"`
		Name   string `json:"Name"`
	} `json:"Job"`
}

type Meta struct {
	MinQuery          string
	MaxQuery          string
	FireTime          string
	ScaleMin          string
	ScaleMax          string
	ScaleCountUp      string
	ScaleCountDown    string
	ScaleCooldown     string
	ScaleCooldownUp   string
	ScaleCooldownDown string
}

type Prometheus struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric struct {
				ExportedJob string `json:"exported_job"`
			} `json:"metric"`
			Value []interface{} `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

type TrigeredAction struct {
	Time      time.Time
	Direction string
}
