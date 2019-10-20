COMMIT = $(shell git rev-parse HEAD)

build:
	go build -o main main.go
run: build
	./main

docker:
	docker build -t wearebrews/photo_backup_receiver:$(COMMIT) -f dockerfiles/receiver.dockerfile .
	docker build -t wearebrews/photo_backup_receiver -f dockerfiles/receiver.dockerfile .
docker-push: docker
	docker push wearebrews/photo_backup_receiver:$(COMMIT)
	docker push wearebrews/photo_backup_receiver

docker-push-dev:
	docker build -t wearebrews/photo_backup_receiver:dev -f dockerfiles/receiver.dockerfile .
	docker push wearebrews/photo_backup_receiver:dev
