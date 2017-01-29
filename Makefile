CWD=$(shell pwd)
VERSION=1.0

.PHONY: clean

deps:
	glide install -v

hostpath-provisioner:
	go build -o hostpath-provisioner hostpath-provisioner.go 

docker-build:
	docker build -t titilambert/hostpath-provisioner:$(VERSION) .

docker-push:
	docker tag titilambert/hostpath-provisioner:$(VERSION) titilambert/hostpath-provisioner:latest
	docker push titilambert/hostpath-provisioner:$(VERSION)
	docker push titilambert/hostpath-provisioner:latest

docker-irun:
	docker run --rm -it --entrypoint=sh titilambert/hostpath-provisioner:$(VERSION)

docker-run:
	docker run --rm titilambert/hostpath-provisioner:$(VERSION)

clean:
	rm -f hostpath-provisioner
