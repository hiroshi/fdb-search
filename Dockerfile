FROM fdb-go as builder
ADD . /root/go/src/github.com/hiroshi/fdb-search
WORKDIR /root/go/src/github.com/hiroshi/fdb-search
RUN go build

FROM ubuntu:16.04
COPY --from=builder /usr/lib/libfdb_c.so /usr/lib/
COPY --from=builder /root/go/src/github.com/hiroshi/fdb-search/fdb-search ./
