FDB_CHECKOUT = release-5.1
IMAGE = fdb:build_go-$(FDB_CHECKOUT)

DOCKER_RUN = docker run --rm -ti \
  -e GOPATH=/root/foundationdb/bindings/go/build \
  -e LD_LIBRARY_PATH=/root/foundationdb/lib \
  -v fdb:/etc/foundationdb \
  -v $$PWD:/usr/lib/go/src/github.com/hiroshi/fdb-search \
  --workdir=/usr/lib/go/src/github.com/hiroshi/fdb-search

test:
	$(DOCKER_RUN) $(IMAGE) \
	  go test -v

start:
	$(DOCKER_RUN) -p 12345:12345 $(IMAGE) \
	  go run main.go


# NOTE: release-5.1 branch for https://github.com/apple/foundationdb/pull/263
fdb-build_go:
	docker build -t $(IMAGE) .

# https://github.com/apple/foundationdb#linux
fdb-build: foundationdb
	(cd foundationdb/build && docker build -t fdb:build-$(FDB_CHECKOUT) .)

foundationdb:
	git clone git@github.com:apple/foundationdb.git
	(cd $@ && git checkout $(FDB_CHECKOUT))
