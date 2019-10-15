package main

import (
	"reflect"
	"testing"

	"github.com/go-test/deep"
	nomad "github.com/hashicorp/nomad/api"
	"github.com/trivago/scalad/structs"
)

func Test_getJobs(t *testing.T) {
	jobs, _ := getJobs()
	//var mapa map[string]*nomad.Job
	tests := []struct {
		name      string
		want      map[string]*nomad.Job
		wantErr   bool
		nomadHost string
	}{
		{name: "Normal funtion", want: jobs, wantErr: false},
		//{name: "No connection", want: mapa, wantErr: true, nomadHost: "http://nomad.service.consul"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getJobs()
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
