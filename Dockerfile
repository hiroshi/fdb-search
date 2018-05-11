FROM fdb:build_go-release-5.1 as builder
ADD . /root/fdb-search
ENV GOPATH=/root/foundationdb/bindings/go/build
WORKDIR /root/fdb-search
RUN go build

FROM ubuntu:15.04
COPY --from=builder /root/foundationdb/lib/libfdb_c.so /usr/lib/
COPY --from=builder /root/fdb-search/fdb-search ./
