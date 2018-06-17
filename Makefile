# docker image
BASE_IMAGE = fdb-go
IMAGE = hiroshi3110/fdb-search:fdb-5.1
docker-build:
	docker build -t $(IMAGE) .

docker-push:
	docker push $(IMAGE)

PORT = 1234
docker-test-run:
	docker run --rm -ti -v fdb-search_fdb:/etc/foundationdb -e PORT=$(PORT) -p $(PORT):$(PORT) $(IMAGE) ./fdb-search

# build a docker image that contain FoundationDB go binding
docker-build-fdb-go:
	docker build -t $(BASE_IMAGE) -f Dockerfile.base .
