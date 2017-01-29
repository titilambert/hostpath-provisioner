CWD=$(shell pwd)
VERSION=1.0

.PHONY: clean

deps:
	glide install -v

hostpath-provisioner:
	go build -o hostpath-provisioner hostpath-provisioner.go 

docker-build:
	docker build -t titilambert/hostpath-provisioner:1.0 .

clean:
	rm -f hostpath-provisioner
