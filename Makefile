BRANCH = "master"
REPONAME = "neo-go"
NETMODE ?= "privnet"
BINARY = "./bin/neo-go"
DESTDIR = ""
SYSCONFIGDIR = "/etc"
BINDIR = "/usr/bin"
SYSTEMDUNIT_DIR = "/lib/systemd/system"
UNITWORKDIR = "/var/lib/neo-go"

DC_FILE=.docker/docker-compose.yml

REPO ?= "$(shell go list -m)"
VERSION ?= "$(shell git describe --tags 2>/dev/null | sed 's/^v//')"
BUILD_FLAGS = "-X '$(REPO)/pkg/config.Version=$(VERSION)'"

IMAGE_REPO=nspccdev/neo-go

# All of the targets are phony here because we don't really use make dependency
# tracking for files
.PHONY: build deps image check-version clean-cluster push-tag push-to-registry \
	run run-cluster test vet lint fmt cover

build: deps
	@echo "=> Building binary"
	@set -x \
		&& export GOGC=off \
		&& export CGO_ENABLED=0 \
		&& go build -trimpath -v -mod=vendor -ldflags $(BUILD_FLAGS) -o bin/vm.wasm ./cli/main.go

neo-go.service: neo-go.service.template
	@sed -r -e 's_BINDIR_$(BINDIR)_' -e 's_UNITWORKDIR_$(UNITWORKDIR)_' -e 's_SYSCONFIGDIR_$(SYSCONFIGDIR)_' $< >$@

install: build neo-go.service
	@echo "=> Installing systemd service"
	@mkdir -p $(DESTDIR)$(SYSCONFIGDIR)/neo-go \
		&& mkdir -p $(SYSTEMDUNIT_DIR) \
		&& cp ./neo-go.service $(SYSTEMDUNIT_DIR) \
		&& cp ./config/protocol.mainnet.yml $(DESTDIR)$(SYSCONFIGDIR)/neo-go \
		&& cp ./config/protocol.privnet.yml $(DESTDIR)$(SYSCONFIGDIR)/neo-go \
		&& cp ./config/protocol.testnet.yml $(DESTDIR)$(SYSCONFIGDIR)/neo-go \
		&& install -m 0755 -t $(BINDIR) $(BINARY) \

postinst: install
	@echo "=> Preparing directories and configs"
	@id neo-go || useradd -s /usr/sbin/nologin -d $(UNITWORKDIR) neo-go \
		&& mkdir -p $(UNITWORKDIR) \
		&& chown -R neo-go:neo-go $(UNITWORKDIR) $(BINDIR)/neo-go \
		&& systemctl enable neo-go.service

image: deps
	@echo "=> Building image"
	@docker build -t $(IMAGE_REPO):latest --build-arg REPO=$(REPO) --build-arg VERSION=$(VERSION) .
	@docker build -t $(IMAGE_REPO):$(VERSION) --build-arg REPO=$(REPO) --build-arg VERSION=$(VERSION) .

image-push:
	@echo "=> Publish image"
	@docker push $(IMAGE_REPO):latest
	@docker push $(IMAGE_REPO):$(VERSION)

check-version:
	git fetch && (! git rev-list ${VERSION})

version:
	@echo ${VERSION}

deps:
	@go mod tidy -v
	@go mod vendor

push-tag:
	git checkout ${BRANCH}
	git pull origin ${BRANCH}
	git tag ${VERSION}
	git push origin ${VERSION}

push-to-registry:
	@docker login -e ${DOCKER_EMAIL} -u ${DOCKER_USER} -p ${DOCKER_PASS}
	@docker tag CityOfZion/${REPONAME}:latest CityOfZion/${REPONAME}:${CIRCLE_TAG}
	@docker push CityOfZion/${REPONAME}:${CIRCLE_TAG}
	@docker push CityOfZion/${REPONAME}

run: build
	${BINARY} node -config-path ./config -${NETMODE}

test:
	@go test ./... -cover

vet:
	@go vet ./...

lint:
	@go list ./... | xargs -L1 golint -set_exit_status

fmt:
	@gofmt -l -w -s $$(find . -type f -name '*.go'| grep -v "/vendor/")

cover:
	@go test -v -race ./... -coverprofile=coverage.txt -covermode=atomic
	@go tool cover -html=coverage.txt -o coverage.html

# --- Environment ---
env_vendor:
	@echo "=> Update vendor"
	@go mod tidy
	@go mod download
	@go mod vendor

env_image: env_vendor
	@echo "=> Building env image"
	@docker build \
		-t env_neo_go_image \
		--build-arg REPO=$(REPO) \
		--build-arg VERSION=$(VERSION) .

env_up:
	@echo "=> Bootup environment"
	@docker-compose -f $(DC_FILE) up -d node_one node_two node_three node_four

env_single:
	@echo "=> Bootup environment"
	@docker-compose -f $(DC_FILE) up -d node_single

env_down:
	@echo "=> Stop environment"
	@docker-compose -f $(DC_FILE) down

env_restart:
	@echo "=> Stop and start environment"
	@docker-compose -f $(DC_FILE) restart

env_clean: env_down
	@echo "=> Cleanup environment"
	@docker volume rm docker_volume_chain
