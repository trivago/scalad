<p align="center">
<img width="250" src="scalad_mascot.png" />
</p>

Env variables:

*    `PORT = "80"`: Port on which the application is going to be listening.
*    `NOMAD_HOST = "http://nomad.service.consul:4646"`: Nomad host endpoint.
*    `NOMAD_REGION = "global"`: Nomad Region.
*    `VAULT_TOKEN = "ljkasdflkjafd"`: Token for vault with permission to access every secret needed for all the scale jobs.
*    `USE_SLACK = "true"`: Flag to enable the use of slack as a system message application.
*    `HTTP_USER = "user"`: User needed for basic auth for endpoints to scale up or down manually.
*    `HTTP_PASS = "password"`: Password needed for basic auth for endpoints to scale up or down manually
*    `METRICS_ENDPOINT = "http://prometheus.yourorg.com/api/v1/query?query="`: Endpoint from where to get the metrics which are going to be used to triger the scale events.

Inside Job file (At taskGroup lvl):
````
meta {
    scaler = "true" Activate the scaler
    min_query = "sum(rate(nomad_client_allocs_cpu_total_ticks{exported_job='scaler-test'}[1m]))by(exported_job) < bool 1" Query that gives the Min threshold for scaling down
    max_query = "sum(rate(nomad_client_allocs_cpu_total_ticks{exported_job='scaler-test'}[1m]))by(exported_job) > bool 2" Query that gives the Max threshold for scaling up
    query_fire_time = "2m" Time the query need to be true before triggering the saling event
    scale_cooldown_down = "20s" Time in cooldown for a scale dow event
    scale_cooldown_up = "25s" Time in cooldown for a scale up event
    scale_count_down = "1" Amount of containers that are going to be removed on a scale down event
    scale_count_up = "2" Amount of containers that are going to be added on a scale up event
    scale_max = "16" Maximun amount of containers
    scale_min = "1" Minimun amount amount of containers
}
````
