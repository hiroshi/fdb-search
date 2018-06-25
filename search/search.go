package search

import (
	// "fmt"
	"log"
	"strings"

	"github.com/apple/foundationdb/bindings/go/src/fdb"
  "github.com/apple/foundationdb/bindings/go/src/fdb/directory"
  // "github.com/apple/foundationdb/bindings/go/src/fdb/subspace"
  "github.com/apple/foundationdb/bindings/go/src/fdb/tuple"
)

const grams = 3
// Key structures:
//   1) Tri-gram str index:
//     dir(dir, "context", context), str, order, id, pos
//   2) Key to clear index
//     dir(dir, "contextId", context, id), order, str
//   3)
//     dir(dir), dir(dir, "context", context)
//     dir(dir), dir(dir, "contextId", context, id)

func contextDirectorySubspace(db fdb.Transactor, dirName string, context string) (directory.DirectorySubspace) {
	dir, err := db.Transact(func (tr fdb.Transaction) (ret interface{}, e error) {
		contextDir, err := directory.CreateOrOpen(tr, []string{dirName, "context", context}, nil)
		if err != nil {
			log.Fatal(err)
		}
		// fmt.Printf("contextDir:%#v\n", contextDir.Bytes())
		dir, err := directory.CreateOrOpen(tr, []string{dirName}, nil)
		if err != nil {
			log.Fatal(err)
		}
		tr.Set(dir.Sub(contextDir), []byte{})

		return contextDir, nil
	})
	if err != nil {
		log.Fatal(err)
	}
	return dir.(directory.DirectorySubspace)
}

func contextIdDirectorySubspace(db fdb.Transactor, dirName string, context string, id string) (directory.DirectorySubspace) {
	dir, err := db.Transact(func (tr fdb.Transaction) (ret interface{}, e error) {
		contextIdDir, err := directory.CreateOrOpen(db, []string{dirName, "contextId", context, id}, nil)
		if err != nil {
			log.Fatal(err)
		}
		dir, err := directory.CreateOrOpen(tr, []string{dirName}, nil)
		if err != nil {
			log.Fatal(err)
		}
		tr.Set(dir.Sub(contextIdDir), []byte{})
		return contextIdDir, nil
	})
	if err != nil {
		log.Fatal(err)
	}
	return dir.(directory.DirectorySubspace)
}

func ClearIndex(dir string, context string, id string) {
	db := fdb.MustOpenDefault()
	contextDirSub := contextDirectorySubspace(db, dir, context)
	contextIdDirSub := contextIdDirectorySubspace(db, dir, context, id)

	_, err := db.Transact(func (tr fdb.Transaction) (ret interface{}, e error) {
		ri := tr.GetRange(contextIdDirSub, fdb.RangeOptions{}).Iterator()
		for ri.Advance() {
			kv := ri.MustGet()
			t, err := contextIdDirSub.Unpack(kv.Key)
			if err != nil {
				log.Fatalf("Uppack failed: %+v", err)
			}
			order := t[0]
			str := t[1]
			key := contextDirSub.Sub(str, order, id)
			tr.ClearRange(key)
		}
		_, err := contextIdDirSub.Remove(tr, []string{})
		if err != nil {
			log.Fatalf("Directory.Remoe(%+v, %+v) failed.", tr, []string{dir, "contextId", context, id})
		}
		return
	})
	if err != nil {
		log.Fatalf("clearIndex failed (%v)", err)
	}
}

func CreateIndex(dir string, context string, order int64, id string, inputText string) {
	ClearIndex(dir, context, id)

	db := fdb.MustOpenDefault()
	contextDirSub := contextDirectorySubspace(db, dir, context)
	contextIdDirSub := contextIdDirectorySubspace(db, dir, context, id)
	// Create index
	runes := []rune(strings.ToLower(inputText))
	_, err := db.Transact(func (tr fdb.Transaction) (ret interface{}, e error) {
		for i, _ := range runes {
			n := grams
			if i + n >  len(runes) {
				n = len(runes) - i
			}
			str := string(runes[i:i+n])
			// Create key for search
			key := contextDirSub.Sub(str, order, id, i)
			tr.Set(key, []byte("\x01"))
			// Create key for clear old search key
			tr.Set(contextIdDirSub.Sub(order, str), []byte("\x01"))
		}
		return
	})
	if err != nil {
		log.Fatalf("createIndex failed (%v)", err)
	}
}

type SearchResultItem struct {
	Id string `json:"id"`
	Pos int `json:"pos"`
}

type SearchResult struct {
	Items []SearchResultItem `json:"items"`
	Count int `json:"count"`
}

type SearchFuture struct {
	Order int64
	Id tuple.TupleElement
	StartPos int
	RuneIndex int
	Future fdb.FutureByteSlice
}

func Search(dir string, context string, term string) SearchResult {
	db := fdb.MustOpenDefault()
	contextDirSub := contextDirectorySubspace(db, dir, context)

	runes := []rune(strings.ToLower(term))
	runeIndex := 0

	firstRunes := runes
	if len(runes) > grams {
		firstRunes = runes[:grams]
	}
	keyBytes := append(append(contextDirSub.Bytes(), 0x02), []byte(string(firstRunes))...)
	beginKey := fdb.Key(keyBytes)
	endBytes, err := fdb.Strinc(keyBytes)
	if err != nil {
		log.Fatalf("fdb.Strinc() failed: %+v.", err)
	}
	endKey := fdb.Key(endBytes)

	futures := []SearchFuture{}
	items := []SearchResultItem{}
	lastMatchId := ""

	for rangeContinue := true; rangeContinue; {
		// NOTE: ruens and futures are shifted as processed to be able to contine on transaction retry
		_, err := db.ReadTransact(func (tr fdb.ReadTransaction) (ret interface{}, e error) {
			for {
				nextRuneIndex := runeIndex + grams
				if runeIndex + grams < len(runes) && len(runes) < runeIndex + grams * 2 {
					nextRuneIndex = len(runes) - grams
				}

				process := func(futures []SearchFuture, future SearchFuture) []SearchFuture {
					if runeIndex + grams < len(runes) {
						str := string(runes[nextRuneIndex : nextRuneIndex + grams])
						nextKey := contextDirSub.Sub(str, future.Order, future.Id, future.StartPos + nextRuneIndex)
						futures = append(futures, SearchFuture{future.Order, future.Id, future.StartPos, nextRuneIndex, tr.Get(nextKey)})
					} else if lastMatchId != future.Id {
						item := SearchResultItem{future.Id.(string), future.StartPos}
						items = append(items, item)
						lastMatchId = future.Id.(string)
					}
					return futures
				}

				if runeIndex == 0 {
					ri := tr.GetRange(fdb.KeyRange{beginKey, endKey}, fdb.RangeOptions{Reverse: true}).Iterator()
					for rangeContinue && len(futures) <= 10000 {
						rangeContinue = ri.Advance()
						if !rangeContinue {
							break
						}
						kv := ri.MustGet()
						endKey = kv.Key
						t, err := contextDirSub.Unpack(kv.Key)
						if err != nil {
							log.Fatalf("Unpack failed: %+v.", err)
						}
						futures = process(futures, SearchFuture{Order: t[1].(int64), Id: t[2], StartPos: int(t[3].(int64))})
					}
				} else {
					nextFutures := futures[:0]
					for _, future := range futures {
						v := future.Future.MustGet()
						if string(v) != "" {
							nextFutures = process(nextFutures, future)
						}
					}
					futures = nextFutures
					if runeIndex + grams > len(runes) {
						break
					}
				}
				runeIndex = nextRuneIndex
			} // for runeIndex
			if rangeContinue {
				runeIndex = 0
			}
			return
		})
		if err != nil {
		    log.Fatalf("search failed (%v)", err)
		}
	}
	return SearchResult{items, len(items)}
}
