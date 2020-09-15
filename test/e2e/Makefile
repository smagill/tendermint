# Makefile for end-to-end tests

build-docker-e2e:
	docker build --tag tendermint/e2e-node -f docker/Dockerfile ../..
.PHONY: build-docker-e2e

app: app/app
.PHONY: app

app/app:
	go build -o app/app ./app/...
