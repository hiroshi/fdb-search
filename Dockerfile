FROM fdb:build-release-5.1
ADD foundationdb /root/foundationdb
# https://github.com/apple/foundationdb/tree/master/bindings/go
# RUN cd /root/foundationdb && make fdb_go
RUN cd /root/foundationdb && make default fdb_go
