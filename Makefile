RUN=go run main.go

run:
	docker run --rm -ti \
	  -e GOPATH=/root/foundationdb/bindings/go/build \
	  -e LD_LIBRARY_PATH=/root/foundationdb/lib \
	  -v fdb:/etc/foundationdb \
	  -v $$PWD:/usr/lib/go/src/github.com/hiroshi/fdb-search \
	  --workdir=/usr/lib/go/src/github.com/hiroshi/fdb-search \
	  -p 12345:12345 \
	  fdb:build_go-$(FDB_CHECKOUT) $(RUN)


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
