
CONTAINER_DIR := /go/src/github.com/simonswine/rocklet
IMAGE_NAME := rocklet-build-image

HACK_DIR ?= hack

HOST ?= rockrobo

# A list of all types.go files in pkg/apis
TYPES_FILES := $(shell find pkg/apis -name types.go)

docker_build_image:
	docker build -t $(IMAGE_NAME) -f Dockerfile.build .

build: version
	go build -o rocklet -ldflags "-X main.gitState=${GIT_STATE} -X main.commitHash=${GIT_COMMIT} -X main.appVersion=${APP_VERSION}"

# Building ARM Binary
build_arm:
	GOOS=linux GOARCH=arm GOARM=7 CC=arm-linux-gnueabihf-gcc CGO_ENABLED=1 go build -o rocklet_arm -ldflags "-X main.gitState=${GIT_STATE} -X main.commitHash=${GIT_COMMIT} -X main.appVersion=${APP_VERSION}"
	# Example building tests for arm: GOOS=linux GOARCH=arm GOARM=7 CC=arm-linux-gnueabihf-gcc CGO_ENABLED=1 go test -c -o rocklet_arm ./pkg/navmap/

# Sync binary to rockrobo
rockrobo_sync:
	rsync --partial --progress rocklet_arm root@$(HOST):/mnt/data/rocklet

# Code generation
#################
# This target runs all required generators against our API types.
generate: $(TYPES_FILES)
	$(HACK_DIR)/update-codegen.sh

generate_verify:
	$(HACK_DIR)/verify-codegen.sh

version:
	$(eval GIT_STATE ?= $(shell if test -z "`git status --porcelain 2> /dev/null`"; then echo "clean"; else echo "dirty"; fi))
	$(eval GIT_COMMIT ?= $(shell git rev-parse HEAD 2> /dev/null))
	$(eval APP_VERSION ?= $(shell cat VERSION 2> /dev/null))
	echo $(APP_VERSION)

docker_%: docker_build_image version
	# create a container
	$(eval CONTAINER_ID := $(shell docker create \
		-i \
		-w $(CONTAINER_DIR) \
		-e GIT_STATE=$(GIT_STATE) \
		-e GIT_COMMIT=$(GIT_COMMIT) \
		-e APP_VERSION=$(APP_VERSION) \
		$(IMAGE_NAME) \
		/bin/bash -c "make $*" \
	))

	# copy stuff into container
	git ls-files | tar cf -  -T - | docker cp - $(CONTAINER_ID):$(CONTAINER_DIR)
	
	# run build inside container
	docker start -a -i $(CONTAINER_ID)

	# copy artifacts over
	docker cp $(CONTAINER_ID):$(CONTAINER_DIR)/rocklet . || /bin/true
	docker cp $(CONTAINER_ID):$(CONTAINER_DIR)/rocklet_arm . || /bin/true

	# remove container
	docker rm $(CONTAINER_ID)
