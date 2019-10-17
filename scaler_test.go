package main

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"reflect"
	"testing"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/trivago/scalad/structs"
)

func BodytoString(body io.ReadCloser) string {
	bodyBytes, err := ioutil.ReadAll(body)
	if err != nil {
		log.Fatal(err)
	}
	return string(bodyBytes)
}

func Test_newScaler(t *testing.T) {
	expect := Scaler{jobMap: make(map[string]structs.JobStruct)}
	tests := []struct {
		name string
		want Scaler
	}{
		{name: "Normal", want: expect},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := newScaler(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newScaler() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestScaler_health(t *testing.T) {
	client := &http.Client{
		Timeout: (time.Second * 10),
	}

	req, err := http.NewRequest("GET", "http://127.0.0.1:8088/", nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := client.Do(req)

	if resp.StatusCode != 200 {
		t.Errorf("health returned wrong status code: got %v want 200", resp.Status)
	}

	expected := "<html>All Good</html>"

	bodyString := BodytoString(resp.Body)
	log.Info(bodyString)
	if bodyString != expected {
		t.Errorf("health returned wrong status code: got %v want %v", bodyString, expected)
	}

}

func TestScaler_stopScallingJob(t *testing.T) {
	region = "dc1"
	client := &http.Client{
		Timeout: (time.Second * 10),
	}

	req, err := http.NewRequest("GET", "http://127.0.0.1:8088/stop-scalling/test-job/1m", nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := client.Do(req)

	if resp.StatusCode != 200 {
		t.Errorf("health returned wrong status code: got %v want 200", resp.Status)
	}

	bodyString := BodytoString(resp.Body)
	expected := "Manually paused: test-job-dc1 for 1m"
	if bodyString != expected {
		t.Errorf("health returned wrong status code: got %v want %v", bodyString, expected)
	}

	req, err = http.NewRequest("GET", "http://127.0.0.1:8088/stop-scalling/test-job/1x", nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err = client.Do(req)

	bodyString = BodytoString(resp.Body)
	expected = "Error parsing time for test-job-dc1 with timer 1x"
	if bodyString != expected {
		t.Errorf("health returned wrong status code: got %v want %v", bodyString, expected)
	}
}

func TestScaler_resumeScallingJob(t *testing.T) {
	region = "dc1"
	client := &http.Client{
		Timeout: (time.Second * 10),
	}

	req, err := http.NewRequest("GET", "http://127.0.0.1:8088/resume-scalling/test-job", nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := client.Do(req)

	if resp.StatusCode != 200 {
		t.Errorf("health returned wrong status code: got %v want 200", resp.Status)
	}

	bodyString := BodytoString(resp.Body)
	expected := "Manually resumed: test-job-dc1"
	if bodyString != expected {
		t.Errorf("health returned wrong status code: got %v want %v", bodyString, expected)
	}

}

func TestScaler_scaleAction(t *testing.T) {
	var postRequest structs.PostRequest
	emptyJSONRequest, _ := json.Marshal(postRequest)

	jsonRequest1 := `{
		"reciever":"esteban",
		"alerts": [{
				"status": "firing",
				"labels": {
					"alertname": "scaleup",
					"client": "#/io.l5d.consul/campus/trv-net-dev-internal/ping-dev2",
					"dc": "campus",
					"region": "dc1",
					"host": "nomad1-dg",
					"instance": "172.20.133.101:9990",
					"job": "linkerd-admin",
					"monitor": "tcs",
					"rt": "http",
					"service": "svc/ping-dev2.dev.tcs.trv.cloud",
					"severity": "critical",
					"tags": ",trv-metrics,trv-env-dev,",
					"allocID": "123456",
					"jobID": "test-job"
				},
				"annotations": {
					"description": "jobname:",
					"summary": "jobname:"
				},
				"startsAt": "2018-12-13T10:01:22.541331374Z",
				"endsAt": "0001-01-01T00:00:00Z",
				"generatorURL": ""
			},
			{
				"status": "firing",
				"labels": {
					"alertname": "scaleup",
					"client": "#/io.l5d.consul/campus/trv-net-dev-internal/ping-dev2",
					"dc": "campus",
					"region": "dc1",
					"host": "nomad1-dg",
					"instance": "172.20.133.101:9990",
					"job": "linkerd-admin",
					"monitor": "tcs",
					"rt": "http",
					"service": "svc/ping-dev2.dev.tcs.trv.cloud",
					"severity": "critical",
					"tags": ",trv-metrics,trv-env-dev,",
					"allocID": "123456",
					"jobID": "test-job"
				},
				"annotations": {
					"description": "jobname:",
					"summary": "jobname:"
				},
				"startsAt": "2018-12-13T10:01:22.541331374Z",
				"endsAt": "0001-01-01T00:00:00Z",
				"generatorURL": ""
			}],
		"version": "1.5"
		}`

	type fields struct {
		jobMap map[string]structs.JobStruct
	}
	//var mapa fields
	//var job structs.JobStruct
	//mapa.jobMap["test-job"] = job

	type args struct {
		body []byte
	}

	var req1 args
	req1.body = emptyJSONRequest

	var req2 args
	req2.body = []byte(jsonRequest1)

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		//{name: "Empty payload request", args: req1, wantErr: false},
		{name: "scale up request", args: req2, wantErr: false},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scaler := &Scaler{
				jobMap: tt.fields.jobMap,
			}
			if err := scaler.scaleAction(tt.args.body); (err != nil) != tt.wantErr {
				t.Errorf("Scaler.scaleAction() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
