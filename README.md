fdb-search (a tentative title)
==========

## Usage example
### Create index
```
$ curl http://localhost:1234/index \
  -F dir="my_app" \
  -F context="user_1" \
  -F order=0 \
  -F id="doc_1" \
  -F text="hello FoundationDB"
Index created for context='user_1', id='doc_1'
```

### Search term
```
$ curl -s 'http://localhost:1234/search?dir=my_app&context=user_1&term=hello'
{"items":[{"id":"doc_1","pos":0}],"count":1}
```

A search result items are returned in reverse of `order` of indexes. If you use timepstamp integers like second from epoc as `order`, you will get latests first.


## Quick start

Start a FoundationDB cluster.
```
$ docker run --init --rm --name=fdb-0 -v fdb-0:/var/lib/foundationdb \
    hiroshi3110/foundationdb:5.1.5-1_ubuntu-16.04 ./start.sh
```

```
$ docker run -ti --rm -v fdb-0:/etc/foundationdb -e PORT=1234 -p 1234:1234 \
    hiroshi3110/fdb-search ./fdb-search
```

For more infomation about the `hiroshi3110/foundationdb:5.1.5-1_ubuntu-16.04` dockder image, see https://github.com/hiroshi/foundationdb-docker.
