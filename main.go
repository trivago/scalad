package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/go-chi/chi"
	"github.com/hashicorp/nomad/api"
	nomad "github.com/hashicorp/nomad/api"
	tparse "github.com/karrick/tparse/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/trivago/scalad/slack"
	"github.com/trivago/scalad/structs"
)

var (
	port                = os.Getenv("PORT")
	nomadHost           = os.Getenv("NOMAD_HOST")
	region              = os.Getenv("NOMAD_REGION")
	vaultToken          = os.Getenv("VAULT_TOKEN")
	useSlack            = os.Getenv("USE_SLACK")
	username            = os.Getenv("HTTP_USER")
	password            = os.Getenv("HTTP_PASS")
	metricsEndpoint     = os.Getenv("METRICS_ENDPOINT")
	infoTime            = os.Getenv("INFO_TIME")
	fireMapTickerEnv    = os.Getenv("FIREMAP_TICKER_SECS")
	scalerTickerEnv     = os.Getenv("SCALER_TICKER_SECS")
	scalerTickerEnvInt  int64
	fireMapTickerEnvInt int64
	lastJobs            map[int64]lastJob
	namespace           = "scalers"
	subsystem           = ""
	scalerLabels        = []string{"name", "region", "direction"}
	apiLabels           = []string{}
	scalerVec           = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: prometheus.BuildFQName(namespace, subsystem, "count"),
			Help: "Scaling jobs",
		},
		scalerLabels,
	)
	apiRequestsVec = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: prometheus.BuildFQName(namespace, subsystem, "apicalls"),
			Help: "Scaling jobs",
		},
		apiLabels,
	)
	mutex            = &sync.Mutex{}
	jobMap           map[string]*nomad.Job
	jobMapMutex      = &sync.Mutex{}
	jobMapScale      map[string]*nomad.Job
	jobMapScaleMutex = &sync.Mutex{}
	jobMetaMap       map[string]*structs.Meta
	jobMetaMapMutex  = &sync.Mutex{}
	fireTimeMap      map[string]*structs.TrigeredAction
	fireTimeMapMutex = &sync.Mutex{}

	scaler Scaler
)

// init function checks that both env variables are set in order to run the scaler.
// These are: nomadAddr -> Address under wich nomad is running
//            port 		-> port in which the application is going to listen.
// This function also register two Vectors with prometheus.
// One for api requests and another for scale operations performed
// by the scaler.
func init() {
	if len(nomadHost) == 0 {
		nomadHost = "http://nomad.service.consul:4646"
	}
	if len(port) == 0 {
		port = ":8080"
	}
	if len(infoTime) == 0 {
		infoTime = "60"
	}
	if len(fireMapTickerEnv) == 0 {
		fireMapTickerEnv = "30"
	}
	if len(scalerTickerEnv) == 0 {
		scalerTickerEnv = "60"
	}
	var err error
	fireMapTickerEnvInt, err = strconv.ParseInt(fireMapTickerEnv, 10, 64)
	if err != nil {
		log.Fatal("Error converting fireMapTicker to int with err: ", err)
	}

	scalerTickerEnvInt, err = strconv.ParseInt(scalerTickerEnv, 10, 64)
	if err != nil {
		log.Fatal("Error converting scalerTicker to int with err: ", err)
	}

	prometheus.MustRegister(scalerVec)
	prometheus.MustRegister(apiRequestsVec)

	jobMap = make(map[string]*nomad.Job)
	jobMapScale = make(map[string]*nomad.Job)
	jobMetaMap = make(map[string]*structs.Meta)
	fireTimeMap = make(map[string]*structs.TrigeredAction)
	lastJobs = make(map[int64]lastJob)
}

// startHTTP function starts the chi router and register all the enpoints availables.
func startHTTP() {
	r := chi.NewMux()

	scaler = newScaler()

	r.Post("/scale", scaler.scale)

	r.Get("/", scaler.health)

	r.Get("/stop-scalling/{jobName}/{timer}", scaler.stopScallingJob)

	r.Get("/resume-scalling/{jobName}", scaler.resumeScallingJob)

	r.Get("/scale-up/{jobName}/{region}", manualScaleUp)
	r.Get("/scale-down/{jobName}/{region}", manualScaleDown)

	r.Get("/info", StatusPage)

	promHandler := promhttp.Handler()
	r.Get("/metrics", promHandler.ServeHTTP)

	// Profiling endpoints. These are disabled to preserver memory.
	/*
		r.Get("/debug/pprof/", pprof.Index)
		r.Get("/debug/pprof/cmdline", pprof.Cmdline)
		r.Get("/debug/pprof/profile", pprof.Profile)
		r.Get("/debug/pprof/symbol", pprof.Symbol)

		// Register pprof handlers
		r.HandleFunc("/debug/pprof/", pprof.Index)
		r.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		r.HandleFunc("/debug/pprof/profile", pprof.Profile)
		r.HandleFunc("/debug/pprof/symbol", pprof.Symbol)

		r.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
		r.Handle("/debug/pprof/heap", pprof.Handler("heap"))
		r.Handle("/debug/pprof/threadcreate", pprof.Handler("threadcreate"))
		r.Handle("/debug/pprof/block", pprof.Handler("block"))
	*/
	http.ListenAndServe(port, r)
}

func checkFiringMap() {
	log.Debug("Checking firingMap")
	fireTimeMapMutex.Lock()
	for mapName, trigger := range fireTimeMap {
		log.Debug(mapName, trigger)
		runTime, err := tparse.AddDuration(trigger.Time, "+"+jobMetaMap[mapName].FireTime)
		if err != nil {
			log.Error("ERROR: JobName: ", mapName)
			log.Error("Can't add trigger.Time and meta.Firetime in checkFiringMap with err: ", err)
			continue
		}
		now := time.Now()
		if now.After(runTime) {
			if trigger.Direction == "up" {
				log.Debug("Scaling up: ", mapName)
				err := scaler.ScaleUp(mapName, region)
				if err != nil {
					log.Error("Error scaling up with err: ", err)
					continue
				}
				delete(fireTimeMap, mapName)

			} else if trigger.Direction == "down" {
				log.Debug("Scaling down: ", mapName)
				err := scaler.ScaleDown(mapName, region)
				if err != nil {
					log.Error("Error scaling up with err: ", err)
					continue
				}
				delete(fireTimeMap, mapName)
			}
		}
	}
	fireTimeMapMutex.Unlock()

}

func addToFiringMap(id string, trigered time.Time, direction string) {
	_, ok := fireTimeMap[id]
	if !ok {
		var trigeredAction structs.TrigeredAction
		trigeredAction.Time = trigered
		trigeredAction.Direction = direction

		fireTimeMapMutex.Lock()
		fireTimeMap[id] = &trigeredAction
		fireTimeMapMutex.Unlock()
		log.Debug("added entry to fireTimeMap -> Direction: ", fireTimeMap[id].Direction, " Trigered: ", fireTimeMap[id].Time)
	}

}

func removeFromFiringMap(id string) {
	_, ok := fireTimeMap[id]
	if !ok {
		fireTimeMapMutex.Lock()
		delete(fireTimeMap, id)
		fireTimeMapMutex.Unlock()
		log.Debug("removed entry from fireTimeMap for ", id)
	}

}

func prometheusQueries(jobMetaMap map[string]*structs.Meta) {
	jobMetaMapMutex.Lock()
	for id, job := range jobMetaMap {
		job.MaxQuery = strings.Replace(job.MaxQuery, "\\", "", -1)
		job.MinQuery = strings.Replace(job.MinQuery, "\\", "", -1)

		maxQuery := metricsEndpoint + job.MaxQuery
		minQuery := metricsEndpoint + job.MinQuery

		log.Debug("Job: ", id)
		log.Debug("MaxQuery query: ", maxQuery)
		maxResult, err := queryPrometheus(maxQuery, job.MaxQuery)
		if err != nil {
			log.Error("Unable to get max result from prometheus with err: ", err, " for job: ", id)
			removeFromFiringMap(id)
			continue
		}

		log.Debug("MaxResult query result: ", maxResult)
		if maxResult {
			addToFiringMap(id, time.Now(), "up")
			continue
		}

		log.Debug("MinQuery query: ", minQuery)
		minResult, err := queryPrometheus(minQuery, job.MinQuery)
		if err != nil {
			log.Error("Unable to get min result from prometheus with err: ", err, " for job: ", id)
			removeFromFiringMap(id)
			continue
		}

		log.Debug("MinResult query result: ", minResult)
		if minResult {
			addToFiringMap(id, time.Now(), "down")
			continue
		}
	}

	jobMetaMapMutex.Unlock()
}

func queryPrometheus(query string, promQuery string) (bool, error) {
	var result structs.Prometheus

	client := &http.Client{
		Timeout: (time.Second * 10),
	}

	u, err := url.Parse(fmt.Sprintf("%s%s", metricsEndpoint, url.QueryEscape(promQuery)))
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		log.Error("Error creating new request with err: ", err)
		return false, err
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Error("Error executing request with err:", err)
		return false, err
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error("Unabel to read resp.Body: ", err)
		return false, err
	}

	if 400 <= resp.StatusCode {
		return false, fmt.Errorf("error response: %s", string(data))
	}

	if err = json.Unmarshal(data, &result); err != nil {
		log.Error("Unable to unmarshall with err: ", err)
		return false, err
	}

	var resultInt int

	if len(result.Data.Result) > 0 {
		if len(result.Data.Result[0].Value) > 0 {
			resultInt, err = strconv.Atoi(result.Data.Result[0].Value[1].(string))
			if err != nil {
				log.Error("Error canverting prometheus response into Int with err: ", err)
				return false, err
			}
		}
	} else {
		return false, fmt.Errorf("Error: lenght of propetheus respond is 0")
	}

	if resultInt != 1 {
		return false, err
	}

	return true, err
}

func checkMeta(jobMap map[string]*api.Job) {
	jobMapScaleMutex.Lock()
	jobMetaMapMutex.Lock()
	defer jobMapScaleMutex.Unlock()
	defer jobMetaMapMutex.Unlock()
	for _, job := range jobMap {
		if job.Meta["scaler"] == "true" {
			jobMapScale[*job.Name] = job
			jobMetaMap[*job.Name] = readMeta(job.Meta)
			log.Debug("Adding ", *job.Name, " to jobMapScale JOB level")
		}
		for _, taskGroup := range job.TaskGroups {
			if taskGroup.Meta["scaler"] == "true" {
				// bug
				// todo: replace with job.Name + task group
				jobMapScale[*job.Name] = job
				jobMetaMap[*job.Name] = readMeta(taskGroup.Meta)
				log.Debug("Adding ", *job.Name, " to jobMapScale TASKGROUP level")
			}
			for _, task := range taskGroup.Tasks {
				if task.Meta["scaler"] == "true" {
					jobMapScale[*job.Name] = job
					jobMetaMap[*job.Name] = readMeta(task.Meta)
					log.Debug("Adding ", *job.Name, " to jobMapScale TASK level")

				}
			}
		}
	}
}

func readMeta(t map[string]string) *structs.Meta {
	var m structs.Meta
	m.MinQuery = t["min_query"]
	m.MaxQuery = t["max_query"]
	m.FireTime = t["query_fire_time"]
	m.ScaleMin = t["scale_min"]
	m.ScaleMax = t["scale_max"]
	m.ScaleCountUp = t["scale_count_up"]
	m.ScaleCooldown = t["scale_count_down"]
	m.ScaleCooldownUp = t["scale_cooldown_up"]
	m.ScaleCooldownDown = t["scale_cooldown_down"]
	return &m
}

func getJobs() (map[string]*nomad.Job, error) {
	jobMap := make(map[string]*nomad.Job)

	nomadClient, err := api.NewClient(&api.Config{Address: nomadHost, TLSConfig: &api.TLSConfig{}})
	if err != nil {
		log.Error("Error creating nomad client with err: ", err)
	}

	options := &api.QueryOptions{AllowStale: true}

	joblist, _, err := nomadClient.Jobs().List(options)
	if err != nil {
		log.Error("Unable to get job list from nomad with err: ", err)
		return nil, err
	}

	jobMapMutex.Lock()
	jobMapScaleMutex.Lock()

	for job := range jobMap {
		delete(jobMap, job)
	}

	for job := range jobMapScale {
		delete(jobMapScale, job)
	}

	jobMapScaleMutex.Unlock()

	for _, job := range joblist {
		value, _, err := nomadClient.Jobs().Info(job.ID, options)
		if err != nil {
			log.Error("Error geting job Info from nomad with err: ", err, " for jobName: ", job.Name)
			continue
		}

		if value.IsPeriodic() == true || *value.Type == "system" || *value.Type == "batch" {
			continue
		}

		jobMap[job.Name] = value
	}

	jobMapMutex.Unlock()

	return jobMap, nil

}

// main function sets the logging formatter, logging level, starts the go routine for the http
// server and waits for a kill signal.
func main() {
	customFormater := new(log.TextFormatter)
	customFormater.FullTimestamp = true
	customFormater.TimestampFormat = "2006-01-02 15:04:05"
	customFormater.ForceColors = true
	log.SetFormatter(customFormater)
	//log.SetLevel(log.InfoLevel)
	log.SetLevel(log.DebugLevel)
	log.Info("Loging to stderr")

	log.Info("Starting scalad....")
	log.Info("Loaded configuration:")
	log.Info("Port:            ", port)
	log.Info("Nomad Host:      ", nomadHost)
	log.Info("Nomad Region:    ", region)
	if len(vaultToken) != 0 {
		log.Info("Vault Token:     ", "************")
	} else {
		log.Info("Vault Token:     ", "EMPTY!!")
	}
	log.Info("Use slack:       ", useSlack)
	log.Info("Http user:       ", username)
	if len(password) != 0 {
		log.Info("Http pass:       ", "**********")
	} else {
		log.Info("Http pass:       ", "EMPTY!!!")
	}
	log.Info("Metrics Endpoint:", metricsEndpoint)

	if useSlack == "true" {
		slack.StartSlackTicker()
	}

	go startHTTP()

	go scalerTicker()

	go fireMapTicker()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	// Block until a signal is received.
	s := <-c
	log.Debug("Got signal:", s)

}

// GetJob function contacts Nomad based on nomadAddr with an jobID and returns the body of this request.
// This requests contains the job definition from nomad that wants to be scaled.
func GetJob(jobID string, region string) (nomad.Job, error) {

	if _, ok := jobMap[jobID]; ok {
		return *jobMap[jobID], nil
	}

	var nomadJob nomad.Job

	client, err := api.NewClient(&api.Config{Address: nomadHost, TLSConfig: &api.TLSConfig{}})
	if err != nil {
		log.Error("Unable to create Nomad client with err: ", err)
		return nomadJob, err
	}

	options := &api.QueryOptions{AllowStale: true}

	nomadJobPointer, _, err := client.Jobs().Info(jobID, options)

	nomadJob = *nomadJobPointer
	if err != nil {
		log.Error("Unable to get job for", jobID, " from nomad with err: ", err)
		return nomadJob, err
	}

	return nomadJob, nil

}

func executeJob(nomadJob nomad.Job) (ok bool, err error) {
	*nomadJob.VaultToken = vaultToken

	nomadClient, err := api.NewClient(&api.Config{Address: nomadHost, TLSConfig: &api.TLSConfig{}})
	if err != nil {
		log.Error("Unable to create Nomad client with err: ", err)
		return false, err
	}

	_, _, err = nomadClient.Jobs().Register(&nomadJob, nil)
	if err != nil {
		return false, err
	}

	return true, nil
}
