package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"testing"

	log "github.com/Sirupsen/logrus"
	"github.com/go-chi/chi"
	"github.com/go-test/deep"
	"github.com/hashicorp/nomad/api"
	nomad "github.com/hashicorp/nomad/api"
	"github.com/trivago/scalad/structs"
)

type syncMessage struct {
}

func init() {
	syncChan := make(chan syncMessage)
	go fakeHTTPServer(syncChan)
	_ = <-syncChan
	log.SetLevel(log.PanicLevel)
}

const (
	testJob = `{
	"Stop": false,
	"Region": "dc1",
	"Namespace": "default",
	"ID": "test-job",
	"ParentID": "",
	"Name": "test-job",
	"Type": "service",
	"Priority": 50,
	"AllAtOnce": false,
	"Datacenters": [
	  "dc1"
	],
	"Constraints": null,
	"Affinities": null,
	"Spreads": null,
	"TaskGroups": [
	  {
		"Name": "test-job",
		"Count": 1,
		"Update": {
		  "Stagger": 30000000000,
		  "MaxParallel": 1,
		  "HealthCheck": "checks",
		  "MinHealthyTime": 30000000000,
		  "HealthyDeadline": 60000000000,
		  "ProgressDeadline": 600000000000,
		  "AutoRevert": true,
		  "AutoPromote": false,
		  "Canary": 1
		},
		"Migrate": {
		  "MaxParallel": 1,
		  "HealthCheck": "checks",
		  "MinHealthyTime": 10000000000,
		  "HealthyDeadline": 300000000000
		},
		"Constraints": null,
		"RestartPolicy": {
		  "Attempts": 2,
		  "Interval": 1800000000000,
		  "Delay": 15000000000,
		  "Mode": "fail"
		},
		"Tasks": [
		  {
			"Name": "test-job",
			"Driver": "docker",
			"User": "",
			"Config": {
			  "image": "artifactory.com/test-job",
			  "port_map": [
				{
				  "http": 80
				}
			  ]
			},
			"Env": null,
			"Vault": null,
			"Templates": null,
			"Constraints": null,
			"Affinities": null,
			"Resources": {
			  "CPU": 100,
			  "MemoryMB": 128,
			  "DiskMB": 0,
			  "IOPS": 0,
			  "Networks": [
				{
				  "Device": "",
				  "CIDR": "",
				  "IP": "",
				  "MBits": 10,
				  "ReservedPorts": null,
				  "DynamicPorts": [
					{
					  "Label": "http",
					  "Value": 0
					},
					{
					  "Label": "grpc",
					  "Value": 0
					}
				  ]
				}
			  ],
			  "Devices": null
			},
			"DispatchPayload": null,
			"Meta": null,
			"KillTimeout": 5000000000,
			"LogConfig": {
			  "MaxFiles": 10,
			  "MaxFileSizeMB": 10
			},
			"Artifacts": null,
			"Leader": false,
			"ShutdownDelay": 15000000000,
			"KillSignal": ""
		  }
		],
		"EphemeralDisk": {
		  "Sticky": false,
		  "SizeMB": 300,
		  "Migrate": false
		},
		"Meta": {
			"max_query": "http_ping_requests_total>bool 5",
			"min_query": "http_ping_requests_total<bool 5",
			"query_fire_time": "2m",
			"scale_cooldown_down": "20s",
			"scale_cooldown_up": "25s",
			"scale_count_down": "1",
			"scale_count_up": "2",
			"scale_max": "16",
			"scale_min": "4",
			"scaler": "true"
		},
		"ReschedulePolicy": {
		  "Attempts": 0,
		  "Interval": 0,
		  "Delay": 30000000000,
		  "DelayFunction": "exponential",
		  "MaxDelay": 3600000000000,
		  "Unlimited": true
		},
		"Affinities": null,
		"Spreads": null
	  }
	],
	"Update": {
	  "Stagger": 30000000000,
	  "MaxParallel": 1,
	  "HealthCheck": "",
	  "MinHealthyTime": 0,
	  "HealthyDeadline": 0,
	  "ProgressDeadline": 0,
	  "AutoRevert": false,
	  "AutoPromote": false,
	  "Canary": 0
	},
	"Periodic": null,
	"ParameterizedJob": null,
	"Dispatched": false,
	"Payload": null,
	"Meta": null,
	"VaultToken": "",
	"Status": "running",
	"StatusDescription": "",
	"Stable": true,
	"Version": 5,
	"SubmitTime": 1565349104838710000,
	"CreateIndex": 5761412,
	"ModifyIndex": 18117344,
	"JobModifyIndex": 18117289
  }`
)

func fakeHTTPServer(syncChan chan syncMessage) {
	log.Info("Running fake nomad server")
	r := chi.NewMux()
	scaler = newScaler()
	r.Get("/v1/{command:[a-z-]+}", jobsMock)
	r.Get("/v1/job/{command:[a-z-]+}", jobMock)
	r.Get("/", scaler.health)
	r.Get("/stop-scalling/{jobName}/{timer}", scaler.stopScallingJob)
	r.Get("/resume-scalling/{jobName}", scaler.resumeScallingJob)
	syncChan <- syncMessage{}
	http.ListenAndServe(":8088", r)
}

func jobsMock(w http.ResponseWriter, r *http.Request) {
	log.Info("Got request for: ", r.URL)
	response := `[{"ID":"test-job","ParentID":"","Name":"test-job","Datacenters":["dc1"],"Type":"service","Priority":50,"Periodic":false,"ParameterizedJob":false,"Stop":false,"Status":"running","StatusDescription":"","JobSummary":{"JobID":"test-job","Namespace":"default","Summary":{"test-job":{"Queued":0,"Complete":6,"Failed":0,"Running":1,"Starting":0,"Lost":0}},"Children":null,"CreateIndex":5118118,"ModifyIndex":18740326},"CreateIndex":5118118,"ModifyIndex":18740329,"JobModifyIndex":18740317,"SubmitTime":1569512423389613911}]`
	fmt.Fprintf(w, "%s", response)
}

func jobMock(w http.ResponseWriter, r *http.Request) {
	log.Info("Got request for: ", r.URL)
	var response string
	if r.URL.String() != "/v1/job/test-job?stale=" {
		response = "job not found"
	} else {
		response = testJob
	}
	fmt.Fprintf(w, "%s", response)
}

func Test_getJobs(t *testing.T) {
	jobs, err := getJobs("http://127.0.0.1:8088")
	if err != nil {
		log.Error("Error getting jobs with err: ", err)
	}
	jobMap = jobs
	var mapa map[string]*nomad.Job
	tests := []struct {
		name    string
		want    map[string]*nomad.Job
		wantErr bool
		address string
	}{
		{name: "Normal funtion", want: jobs, wantErr: false, address: "http://127.0.0.1:8088"},
		{name: "Nomad address not set", want: mapa, wantErr: true, address: ""},
		{name: "Wrong Nomad address", want: mapa, wantErr: true, address: "%foo.html"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getJobs(tt.address)
			if (err != nil) != tt.wantErr {
				t.Errorf("getJobs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getJobs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_readMeta(t *testing.T) {
	expected := structs.Meta{
		MinQuery:          "1",
		MaxQuery:          "2",
		FireTime:          "3",
		ScaleMin:          "4",
		ScaleMax:          "5",
		ScaleCountUp:      "6",
		ScaleCountDown:    "7",
		ScaleCooldownUp:   "8",
		ScaleCooldownDown: "9",
	}
	var testMap map[string]string
	testMap = make(map[string]string)
	testMap["min_query"] = "1"
	testMap["max_query"] = "2"
	testMap["query_fire_time"] = "3"
	testMap["scale_min"] = "4"
	testMap["scale_max"] = "5"
	testMap["scale_count_up"] = "6"
	testMap["scale_count_down"] = "7"
	testMap["scale_cooldown_up"] = "8"
	testMap["scale_cooldown_down"] = "9"

	got := readMeta(testMap)
	if err := deep.Equal(got, &expected); err != nil {
		t.Error("Wrong type of response", err)
	}
}

func Test_checkMeta(t *testing.T) {
	jobs, _ := getJobs("http://127.0.0.1:8088")
	type args struct {
		jobMap map[string]*api.Job
	}

	name := "TestJob"
	mapa := map[string]*api.Job{
		"job2": &api.Job{
			Meta: map[string]string{
				"scaler": "true",
			},
			Name: &name,
			TaskGroups: []*api.TaskGroup{
				{
					Meta: map[string]string{
						"scaler": "true",
					},
					Tasks: []*api.Task{
						{
							Meta: map[string]string{
								"scaler": "true",
							},
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name string
		args args
	}{
		{name: "Nomal", args: args{jobMap: jobs}},
		{name: "Scaler set to true", args: args{jobMap: mapa}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkMeta(tt.args.jobMap)
		})
	}
}

func Test_queryPrometheus(t *testing.T) {
	metricsEndpoint = `http://demo.robustperception.io:9090/api/v1/query?query=`
	type args struct {
		query     string
		promQuery string
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{name: "Normal", args: args{query: "http_ping_requests_total>bool 5", promQuery: "http_ping_requests_total>bool 5"}, want: true, wantErr: false},
		{name: "Query Result != 1", args: args{query: "http_ping_requests_total", promQuery: "http_ping_requests_total"}, want: false, wantErr: false},
		{name: "Response Html code > 400", args: args{query: "http_ping_requests_total", promQuery: "%wrong"}, want: false, wantErr: true},

		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := queryPrometheus(tt.args.query, tt.args.promQuery)
			if (err != nil) != tt.wantErr {
				t.Errorf("queryPrometheus() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("queryPrometheus() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_prometheusQueries(t *testing.T) {
	metricsEndpoint = `http://demo.robustperception.io:9090/api/v1/query?query=`
	type args struct {
		jobMetaMap map[string]*structs.Meta
	}
	jobMetaMapTest := map[string]*structs.Meta{
		"test-job": &structs.Meta{
			MinQuery: "http_ping_requests_total>bool 5", //true
			MaxQuery: "http_ping_requests_total<bool 5", //false
		},
	}
	jobMetaMapTest2 := map[string]*structs.Meta{
		"test-job": &structs.Meta{
			MinQuery: "http_ping_requests_total<bool 5", //false
			MaxQuery: "http_ping_requests_total>bool 5", //true
		},
	}
	jobMetaMapTest3 := map[string]*structs.Meta{
		"test-job": &structs.Meta{
			MinQuery: "http_ping_requests_total>bool 5", //true
			MaxQuery: "http_ping_requests_total>bool 5", //true
		},
	}
	jobMetaMapTest4 := map[string]*structs.Meta{
		"test-job": &structs.Meta{
			MinQuery: "http_ping_requests_total<bool 5", //false
			MaxQuery: "http_ping_requests_total<bool 5", //false
		},
	}

	jobMetaMapTest5 := map[string]*structs.Meta{
		"test-job": &structs.Meta{
			MaxQuery: "%wrong", //error
			MinQuery: "%wrong", //error
		},
	}
	jobMetaMapTest6 := map[string]*structs.Meta{
		"test-job": &structs.Meta{
			MaxQuery: "http_ping_requests_total>bool 5", //true
			MinQuery: "%wrong",                          //error
		},
	}
	jobMetaMapTest7 := map[string]*structs.Meta{
		"test-job": &structs.Meta{
			MaxQuery: "http_ping_requests_total<bool 5", //false
			MinQuery: "%wrong",                          //error
		},
	}
	jobMetaMapTest8 := map[string]*structs.Meta{
		"test-job": &structs.Meta{
			MaxQuery: "%wrong",                          //error
			MinQuery: "http_ping_requests_total>bool 5", //true
		},
	}
	jobMetaMapTest9 := map[string]*structs.Meta{
		"test-job": &structs.Meta{
			MaxQuery: "%wrong",                          //false
			MinQuery: "http_ping_requests_total<bool 5", //error
		},
	}

	tests := []struct {
		name string
		args args
	}{
		{name: "Normal", args: args{jobMetaMapTest}},
		{name: "Error on query", args: args{jobMetaMapTest2}},
		{name: "queryPrometheus ", args: args{jobMetaMapTest3}},
		{name: "queryPrometheus ", args: args{jobMetaMapTest4}},
		{name: "queryPrometheus ", args: args{jobMetaMapTest5}},
		{name: "queryPrometheus ", args: args{jobMetaMapTest6}},
		{name: "queryPrometheus ", args: args{jobMetaMapTest7}},
		{name: "queryPrometheus ", args: args{jobMetaMapTest8}},
		{name: "queryPrometheus ", args: args{jobMetaMapTest9}},

		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prometheusQueries(tt.args.jobMetaMap)
		})
	}
}

func Test_removeFromFiringMap(t *testing.T) {
	type args struct {
		id string
	}
	tests := []struct {
		name string
		args args
	}{
		{name: "Normal", args: args{id: "test-job"}},
		{name: "Normal", args: args{id: "pepe"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			removeFromFiringMap(tt.args.id)
		})
	}
}

func TestGetJob(t *testing.T) {
	address := "http://127.0.0.1:8088"

	var job nomad.Job
	_ = json.Unmarshal([]byte(testJob), &job)
	type args struct {
		address string
		jobID   string
		region  string
	}
	tests := []struct {
		name    string
		args    args
		want    nomad.Job
		wantErr bool
	}{
		{name: "Normal", args: args{address: address, jobID: "test-job", region: "dc1"}, want: job, wantErr: false},
		{name: "Not-existent job", args: args{address: address, jobID: "not-existen", region: "dc1"}, want: nomad.Job{}, wantErr: true},
		//{name: "Wrong nomad address", args: args{address: "%foo.html", jobID: "test-job", region: "dc1"}, want: nomad.Job{}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetJob(tt.args.address, tt.args.jobID, tt.args.region)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetJob() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetJob() = %v, want %v", got, tt.want)
			}
		})
	}
}
