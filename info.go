package main

import (
	"fmt"
	"net/http"
	"time"
)

// LastJobs function updates the last20Jobs array with the last 20 jobs executed.
func LastJobs(jobID string, region string, direction string, time time.Time) {
	for i := 0; i < 19; i++ {
		last20Jobs[i] = last20Jobs[i+1]
	}
	last20Jobs[19] = "<td>" + jobID + "</td><td>" + region + "</td><td>" + direction + "</td><td>" + time.String() + "</td></tr>"
}

// StatusPage function returns an html page displaying the last 20 scalling operations performed.
func StatusPage(w http.ResponseWriter, r *http.Request) {
	message := `<html>
<header>
<style>
table {
    font-family: arial, sans-serif;
    border-collapse: collapse;
    width: 100%;
}

td, th {
    border: 1px solid #dddddd;
    text-align: left;
    padding: 18px;
}

tr:nth-child(even) {
    background-color: #dddddd;
}
</style>
<h1>TCS-Scaler Info</h1>
</header>
<h2>Last request recieved:</h2>
<table>
<tr>
	<th>JobID</th>
	<th>Region</th>
	<th>Alertname</th>
	<th>Recieved At</th>
</tr>
<tr>
`
	fmt.Fprintf(w, "%s", message)
	for i := 0; i < 20; i++ {
		fmt.Fprintf(w, "%s", last20Jobs[i])
	}
	fmt.Fprintf(w, "</table>")
	fmt.Fprintf(w, "</html>")

}
