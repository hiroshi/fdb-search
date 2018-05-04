RUN=go run main.go


run2:
	docker run --rm -ti \
	  -e GOPATH=/root/foundationdb/bindings/go/build \
	  -e LD_LIBRARY_PATH=/root/foundationdb/lib \
	  -v fdb:/etc/foundationdb \
	  -v $$PWD:/usr/lib/go/src/github.com/hiroshi/fdb-search \
	  --workdir=/usr/lib/go/src/github.com/hiroshi/fdb-search \
	  5f3abdb55289 $(RUN)

run:
	docker run --rm -ti \
	  -e GOPATH=/root/foundationdb/bindings/go/build \
	  -e LD_LIBRARY_PATH=/root/foundationdb/lib \
	  -v fdb:/etc/foundationdb \
	  -v $$PWD/../foundationdb:/root/foundationdb \
	  -v $$PWD:/usr/lib/go/src/github.com/hiroshi/fdb-search \
	  --workdir=/usr/lib/go/src/github.com/hiroshi/fdb-search \
	  fdb-go-dev $(RUN)

# build-fdb-go:
# 	docker build . -t hiroshi/foundationdb-client-go:5.1.5-1_ubuntu-16.04 -t fdb-go


# NOTE: release-5.1 branch for https://github.com/apple/foundationdb/pull/263
FDB_CHECKOUT = release-5.1
fdb-build_go:
	docker build -t fdb:build_go-$(FDB_CHECKOUT) .

# https://github.com/apple/foundationdb#linux
fdb-build: foundationdb
	(cd foundationdb/build && docker build -t fdb:build-$(FDB_CHECKOUT) .)

foundationdb:
	git clone git@github.com:apple/foundationdb.git
	(cd $@ && git checkout $(FDB_CHECKOUT))
