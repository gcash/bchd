APPNAME = bchd
CLINAME = bchctl
OUTDIR = pkg

# Allow user to override cross compilation scope
OSARCH ?= darwin/386 darwin/amd64 dragonfly/amd64 freebsd/386 freebsd/amd64 freebsd/arm linux/386 linux/amd64 linux/arm netbsd/386 netbsd/amd64 netbsd/arm openbsd/386 openbsd/amd64 plan9/386 plan9/amd64 solaris/amd64 windows/386 windows/amd64
DIRS ?= darwin_386 darwin_amd64 dragonfly_amd64 freebsd_386 freebsd_amd64 freebsd_arm linux_386 linux_amd64 linux_arm netbsd_386 netbsd_amd64 netbsd_arm openbsd_386 openbsd_amd64 plan9_386 plan9_amd64 solaris_amd64 windows_386 windows_amd64

all:
	go build .
	go build ./cmd/bchctl

compile:
	gox -osarch="$(OSARCH)" -output "$(OUTDIR)/$(APPNAME)-{{.OS}}_{{.Arch}}/$(APPNAME)"
	gox -osarch="$(OSARCH)" -output "$(OUTDIR)/$(APPNAME)-{{.OS}}_{{.Arch}}/$(CLINAME)" ./cmd/bchctl
	@for dir in $(DIRS) ; do \
		(cp README.md $(OUTDIR)/$(APPNAME)-$$dir/README.md) ;\
		(cp LICENSE $(OUTDIR)/$(APPNAME)-$$dir/LICENSE) ;\
		(cp sample-bchd.conf $(OUTDIR)/$(APPNAME)-$$dir/sample-bchd.conf) ;\
		(cd $(OUTDIR) && zip -q $(APPNAME)-$$dir.zip -r $(APPNAME)-$$dir) ;\
		echo "make $(OUTDIR)/$(APPNAME)-$$dir.zip" ;\
	done

install:
	go install .
	go install ./cmd/bchctl

uninstall:
	go clean -i
	go clean -i ./cmd/bchctl

docker:
	docker build -t $(APPNAME) .


protoc-go:
	PATH="${GOPATH}/bin:${PATH}" protoc -I=bchrpc/ bchrpc/bchrpc.proto --go_out=plugins=grpc:bchrpc/pb

protoc-py:
	# python -m pip install grpcio-tools
	python -m grpc_tools.protoc -I=bchrpc/ --python_out=bchrpc/pb-py --grpc_python_out=bchrpc/pb-py bchrpc/bchrpc.proto

protoc-js:
	protoc -I=bchrpc/ \
		--plugin=protoc-gen-ts=$(HOME)/node_modules/.bin/protoc-gen-ts \
		--js_out=import_style=commonjs,binary:bchrpc/pb-js \
		--ts_out=service=true:bchrpc/pb-js \
		bchrpc/bchrpc.proto

protoc-all:
	protoc -I=bchrpc/ bchrpc/bchrpc.proto --go_out=plugins=grpc:bchrpc/pb
	python -m grpc_tools.protoc -I=bchrpc/ --python_out=bchrpc/pb-py --grpc_python_out=bchrpc/pb-py bchrpc/bchrpc.proto
	protoc -I=bchrpc/\
		--plugin=protoc-gen-ts=$(HOME)/node_modules/.bin/protoc-gen-ts \
		--js_out=import_style=commonjs,binary:bchrpc/pb-js \
		--ts_out=service=true:bchrpc/pb-js \
		bchrpc/bchrpc.proto

##
## Snowglobe
##

.DEFAULT_GOAL := help

DOCKER_PROFILE ?= sherpacash

DOCKER_SNOWGLOBED_IMAGE_NAME ?= snowglobed
DOCKER_SHERPAD_IMAGE_NAME ?= sherpad

TAG ?= ""
# TAG ?= $(shell git describe --tags --abbrev=0)
HASH ?= $(shell git rev-parse --short --abbrev=8 HEAD)
BRANCH ?= $(shell git rev-parse --abbrev-ref HEAD)

SHERPA_DB ?= sherpa_dev
SHERPA_DB_USER ?= root
SHERPA_DB_PASSWORD ?= pass
SHERPA_DB_HOST ?= mysql
SHERPA_DB_PORT ?= 3306

SHERPA_DEV_MYSQL_DSN ?= mysql://$(SHERPA_DB_USER):$(SHERPA_DB_PASSWORD)@tcp($(SHERPA_DB_HOST):$(SHERPA_DB_PORT))/sherpa_dev
SHERPA_TEST_MYSQL_DSN ?= mysql://$(SHERPA_DB_USER):$(SHERPA_DB_PASSWORD)@tcp($(SHERPA_DB_HOST):$(SHERPA_DB_PORT))/sherpa_test
SHERPA_MYSQL_DSN ?= $(SHERPA_DEV_MYSQL_DSN)

.PHONY: help
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

##
## Build
##

.PHONY: snowglobed push_snowglobed
snowglobed: ## Create Docker image for snowglobed
	# We always tag the image with the hash and branch name
	docker build -t $(DOCKER_PROFILE)/$(DOCKER_SNOWGLOBED_IMAGE_NAME):$(HASH) -f Dockerfile.distroless .
	docker tag $(DOCKER_PROFILE)/$(DOCKER_SNOWGLOBED_IMAGE_NAME):$(HASH) $(DOCKER_PROFILE)/$(DOCKER_SNOWGLOBED_IMAGE_NAME):$(BRANCH)

	# We also tag the image with the git tag if present and with "latest" if this is master
	if [ "$(TAG)" != "" ]; then docker tag $(DOCKER_PROFILE)/$(DOCKER_SNOWGLOBED_IMAGE_NAME):$(HASH) $(DOCKER_PROFILE)/$(DOCKER_SNOWGLOBED_IMAGE_NAME):$(TAG); fi
	if [ "$(BRANCH)" == "master" ]; then docker tag $(DOCKER_PROFILE)/$(DOCKER_SNOWGLOBED_IMAGE_NAME):$(HASH) $(DOCKER_PROFILE)/$(DOCKER_SNOWGLOBED_IMAGE_NAME):latest; fi

push_snowglobed: ## Create Docker poller service
	docker push $(DOCKER_PROFILE)/$(DOCKER_SNOWGLOBED_IMAGE_NAME):$(HASH)
	docker push $(DOCKER_PROFILE)/$(DOCKER_SNOWGLOBED_IMAGE_NAME):$(BRANCH)
	if [ "$(TAG)" != "" ]; then docker push $(DOCKER_PROFILE)/$(DOCKER_SNOWGLOBED_IMAGE_NAME):$(TAG); fi
	if [ "$(BRANCH)" == "master" ]; then docker push $(DOCKER_PROFILE)/$(DOCKER_SNOWGLOBED_IMAGE_NAME):latest; fi

##
## Tests
##

.PHONY: test_env stop_test_env tests avalanche_tests

test_env: stop_test_env ## Create a fresh test env
	docker-compose -f docker-compose.yml -f docker-compose.test.yml up -d
	docker-compose -f docker-compose.yml -f docker-compose.test.yml run \
		-v $(shell pwd)/snowglobe/config/migrations/:/migrations/ migration \
		-verbose -path="/migrations/" -database="$(SHERPA_TEST_MYSQL_DSN)" up

stop_test_env: ## Tear down the test env
	docker-compose -f docker-compose.yml -f docker-compose.test.yml down

tests: ## Run tests
	go test -i ./...
	go test -v -cover -coverprofile=coverage.out ./...
	go tool cover -html=./coverage.out

avalanche_tests: ## Run avalanche related tests
	go test -i ./...
	go test -v -cover -coverprofile=coverage.out ./avalanche/... ./snowglobe/...
	go tool cover -html=./coverage.out

##
## Dev
##

.PHONY: dev_env stop_dev_env rebuild_dev_env

dev_env: ## Setup a development environment
	docker-compose -f docker-compose.yml -f docker-compose.dev.yml up -d
	sleep 5
	docker-compose -f docker-compose.yml -f docker-compose.dev.yml run \
		-v $(shell pwd)/snowglobe/config/migrations/:/migrations/ migration \
		-verbose -path="/migrations/" -database="$(SHERPA_DEV_MYSQL_DSN)" up

stop_dev_env: ## Tear down the dev env
	docker-compose -f docker-compose.yml -f docker-compose.dev.yml down

rebuild_dev_env: stop_dev_env dev_env ## Stop the existing dev env and create a new one

##
## Database
##

.PHONY: connect_to_db migrate_up migrate_down

connect_to_db: ## Connect to the database using a Docker image
	docker run --rm -it mysql:8 mysql \
	-D $(SHERPA_DB) \
	-h $(SHERPA_DB_HOST) \
	-P $(SHERPA_DB_PORT) \
	-u $(SHERPA_DB_USER) \
	-p$(SHERPA_DB_PASSWORD)

migrate_up: ## Migrate database up
	docker run \
		--network host \
		-v $(shell pwd)/snowglobe/config/migrations/:/migrations \
		migrate/migrate:v4.7.0 \
		-verbose \
    -path=/migrations/ \
		-database "mysql://${SHERPA_MYSQL_DSN}" \
		up

migrate_down: ## Migrate database down
	go get -tags 'mysql' -u github.com/golang-migrate/migrate/cmd/migrate
	migrate -source file://config/migrations -database "mysql://${SHERPA_MYSQL_DSN}" down
