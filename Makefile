# NOTE: release-5.1 branch for https://github.com/apple/foundationdb/pull/263
BASE_IMAGE = fdb-go
FDB_CLUSTER_FILE_VOLUME = fdb-tri
PORT = 1234

# Development
DOCKER_RUN = docker run --rm -ti \
  -v $(FDB_CLUSTER_FILE_VOLUME):/etc/foundationdb \
  -v $$PWD:/usr/lib/go/src/github.com/hiroshi/fdb-search \
  --workdir=/usr/lib/go/src/github.com/hiroshi/fdb-search

test:
	$(DOCKER_RUN) $(BASE_IMAGE) \
	  /usr/lib/go/bin/go test -v

run-dev_server:
	$(DOCKER_RUN) -p $(PORT):$(PORT) -e PORT=$(PORT) $(BASE_IMAGE) \
	  go run main.go

# docker image
IMAGE = hiroshi3110/fdb-search:fdb-5.1
docker-build:
	docker build -t $(IMAGE) .

docker-push:
	docker push $(IMAGE)

docker-test-run:
	docker run --rm -ti -v fdb:/etc/foundationdb -e PORT=$(PORT) -p $(PORT):$(PORT) $(IMAGE) ./fdb-search


# build a docker image that contain FoundationDB go binding
docker-build-fdb-go:
	docker build -t $(BASE_IMAGE) -f Dockerfile.base .
