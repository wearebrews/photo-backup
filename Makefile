COMMIT = $(shell git rev-parse HEAD)

build:
	go build -o main main.go
run: build
	./main

docker:
	docker build -t wearebrews/photo-backup-receiver:$(COMMIT) -f dockerfiles/receiver.dockerfile .
	docker build -t wearebrews/photo-backup-receiver -f dockerfiles/receiver.dockerfile .
docker-push: docker
	docker push wearebrews/photo-backup-receiver:$(COMMIT)
	docker push wearebrews/photo-backup-receiver

docker-push-dev:
	docker build -t wearebrews/photo-backup-receiver:dev -f dockerfiles/receiver.dockerfile .
	docker push wearebrews/photo-backup-receiver:dev
