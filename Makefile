CWD=$(shell pwd)
VERSION=1.0

deps:
	glide install -v

build: 
	go build -o hostpath-provisioner hostpath-provisioner.go 

docker-build:
	docker build -t titilambert/hostpath-provisioner:1.0 .
