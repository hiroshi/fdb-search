# NOTE: release-5.1 branch for https://github.com/apple/foundationdb/pull/263
FDB_CHECKOUT = release-5.1
BASE_IMAGE = fdb:build_go-$(FDB_CHECKOUT)

# Development
DOCKER_RUN = docker run --rm -ti \
  -e GOPATH=/root/foundationdb/bindings/go/build \
  -e LD_LIBRARY_PATH=/root/foundationdb/lib \
  -v fdb:/etc/foundationdb \
  -v $$PWD:/usr/lib/go/src/github.com/hiroshi/fdb-search \
  --workdir=/usr/lib/go/src/github.com/hiroshi/fdb-search

test:
	$(DOCKER_RUN) $(BASE_IMAGE) \
	  go test -v

run-dev_server:
	$(DOCKER_RUN) -p 12345:12345 $(BASE_IMAGE) \
	  go run main.go

# docker image
IMAGE = hiroshi3110/fdb-search:fdb-5.1
docker-build:
	docker build -t $(IMAGE) .

docker-push:
	docker push $(IMAGE)

docker-test-run:
	docker run --rm -ti -v fdb:/etc/foundationdb -p 12345:12345 $(IMAGE) ./fdb-search


# Prerequite docker images
docker-fdb-build_go:
	docker build -t $(BASE_IMAGE) -f Dockerfile.base .

# https://github.com/apple/foundationdb#linux
docker-fdb-build: foundationdb/build/Dockerfile
	cd foundationdb/build && docker build -t fdb:build-$(FDB_CHECKOUT) .

foundationdb/build/Dockerfile: foundationdb/build
	cd foundationdb/build && curl -O https://raw.githubusercontent.com/apple/foundationdb/$(FDB_CHECKOUT)/build/Dockerfile

foundationdb/build:
	mkdir -p $@
