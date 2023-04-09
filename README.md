# OpenStack Instance Stats

OpenStack instance stats is a small Golang project designed to scrap the OpenStack Nova Diagonstics API and store the resulting metrics in an Influxdb TSDB. It is intended to be run in a Kubernetes cluster but can be run stand alone in CRI (docker for example).

## Known Limitions

At this time the program only supports a pretty limited set of situations. 

* Older Openstack with Nova Microservices version < 2.48
* Qemu Hypervisor
* Admin access to the project

If your application doesn't match these exact situations this won't work for you. 

I do plan to add support for the +2.48 microservice in a future release. But this is very much dependant on my own OpenStack deployment getting upgraded. Which is likely not to happen before Q4 2023. 

## Motivations

This project is the result of growing increasingly frustrated with a very non-performant bash script that collected some of these metrics. When it was taking more than 60 seconds to run, I needed to do something very different this is the result. This go service runs comfortably in 10m/30Mi of Cpu/Memory. Vs the bash script that was using 8 cores at 100% cpu for 60seconds per run. 

