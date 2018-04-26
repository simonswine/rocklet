
CONTAINER_DIR := /go/src/github.com/simonswine/rocklet
IMAGE_NAME := rocklet-build-image

HACK_DIR ?= hack

# A list of all types.go files in pkg/apis
TYPES_FILES := $(shell find pkg/apis -name types.go)

docker_build_image:
	docker build -t $(IMAGE_NAME) -f Dockerfile.build .

# Building ARM Binary
build_arm:
	GOOS=linux GOARCH=arm GOARM=7 CC=arm-linux-gnueabihf-gcc CGO_ENABLED=1 go build -o rocklet_arm
	# Example building tests for arm: GOOS=linux GOARCH=arm GOARM=7 CC=arm-linux-gnueabihf-gcc CGO_ENABLED=1 go test -c -o rocklet_arm ./pkg/navmap/

# Sync binary to rockrobo
rockrobo_sync:
	rsync --partial --progress rocklet_arm root@rockrobo:/mnt/data/rocklet

# Code generation
#################
# This target runs all required generators against our API types.
generate: $(TYPES_FILES)
	$(HACK_DIR)/update-codegen.sh

generate_verify:
	$(HACK_DIR)/verify-codegen.sh

docker_%: docker_build_image
	# create a container
	$(eval CONTAINER_ID := $(shell docker create \
		-i \
		-w $(CONTAINER_DIR) \
		$(IMAGE_NAME) \
		/bin/bash -c "make $*" \
	))

	# copy stuff into container
	git ls-files | tar cf -  -T - | docker cp - $(CONTAINER_ID):$(CONTAINER_DIR)
	
	# run build inside container
	docker start -a -i $(CONTAINER_ID)

	# copy artifacts over
	docker cp $(CONTAINER_ID):$(CONTAINER_DIR)/rocklet_arm .

	# remove container
	docker rm $(CONTAINER_ID)
