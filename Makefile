PROTO_FILE=modify.proto
PROTO_GENERATED_FILES_PATH=pkg/rpc
VERSION ?= v0.0.0
LDFLAGS="-X 'main.version=$(VERSION)'"
CONTROLLER_IMG ?= vcr.vngcloud.vn/60108-cuongdm3/vngcloud-csi-volume-modifier
.PHONY: all
all: build

.PHONY: build
build:
	go build -o bin/main -ldflags ${LDFLAGS} cmd/main.go

.PHONY: proto
proto:
	protoc --go_out=$(PROTO_GENERATED_FILES_PATH) --go_opt=paths=source_relative --go-grpc_out=$(PROTO_GENERATED_FILES_PATH) --go-grpc_opt=paths=source_relative $(PROTO_FILE)

.PHONY: test
test:
	go test ./... -race

.PHONY: clean
clean:
	rm -rf bin/


.PHONY: check
check: check-proto

.PHONY: linux/$(ARCH) bin/vngcloud-csi-volume-modifier
linux/$(ARCH): bin/vngcloud-csi-volume-modifier
bin/vngcloud-csi-volume-modifier: | bin
	CGO_ENABLED=0 GOOS=linux GOARCH=$(ARCH) go build -mod=mod -ldflags ${LDFLAGS} -o bin/vngcloud-csi-volume-modifier ./cmd

.PHONY: check-proto
check-proto:
	$(eval TMPDIR := $(shell mktemp -d))
	protoc --go_out=$(TMPDIR) --go_opt=paths=source_relative --go-grpc_out=$(TMPDIR) --go-grpc_opt=paths=source_relative $(PROTO_FILE)
	diff -r $(TMPDIR) $(PROTO_GENERATED_FILES_PATH) || (printf "\nThe proto file seems to have been modified. PLease run `make proto`."; exit 1)
	rm -rf $(TMPDIR)

.PHONY: docker-build
docker-build: ## Build the docker image for controller-manager
	# !IMPORTANT: remember `mkdir -p bin` before running this command
	docker build -f Dockerfile . -t $(CONTROLLER_IMG):$(VERSION)

.PHONY: docker-push
docker-push: ## Build the docker image for controller-manager
	docker image push $(CONTROLLER_IMG):$(VERSION)