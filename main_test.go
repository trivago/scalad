package main

import (
	"reflect"
	"testing"

	"github.com/go-test/deep"
	"github.com/hashicorp/nomad/api"
	nomad "github.com/hashicorp/nomad/api"
	"github.com/trivago/scalad/structs"
)

func Test_getJobs(t *testing.T) {
	jobs, _ := getJobs(nomadHost)
	jobMap = jobs
	var mapa map[string]*nomad.Job
	tests := []struct {
		name    string
		want    map[string]*nomad.Job
		wantErr bool
		address string
	}{
		{name: "Normal funtion", want: jobs, wantErr: false, address: "http://nomad.service.consul:4646"},
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
	jobs, _ := getJobs(nomadHost)
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

/*
max_query:"sum(rate(nginx_http_requests_total{exported_host="tcs-nginx-fpm.dev.tcs.trv.cloud",status="200"}[1m]))by(exported_host) > bool 5 "
min_query:"sum(rate(nginx_http_requests_total{exported_host="tcs-nginx-fpm.dev.tcs.trv.cloud",status="200"}[1m]))by(exported_host) < bool 0 "
query_fire_time:"2m"
scale_cooldown_down:"20s"
scale_cooldown_up:"25s"
scale_count_down:"1"
scale_count_up:"2"
scale_max:"16"
scale_min:"4"
scaler:"true"
*/

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

/*func TestGetJob(t *testing.T) {
	type args struct {
		jobID  string
		region string
	}
	tests := []struct {
		name    string
		args    args
		want    nomad.Job
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetJob(tt.args.jobID, tt.args.region)
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
*/
