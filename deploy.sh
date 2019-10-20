#!/bin/bash

VERSION=$(git rev-parse HEAD)
NAMESPACE=photo-backup
echo "$DOCKER_PASSWORD" | docker login -u "$DOCKER_USERNAME" --password-stdin
make docker-push
kubectl apply -n $NAMESPACE -f kubernetes/
kubectl set image -n $NAMESPACE deployment/receiver receiver=wearebrews/receiver:$VERSION 

