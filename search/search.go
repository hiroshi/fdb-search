package search

import (
	// "fmt"
	"log"
	"strings"

	"github.com/apple/foundationdb/bindings/go/src/fdb"
  "github.com/apple/foundationdb/bindings/go/src/fdb/directory"
  "github.com/apple/foundationdb/bindings/go/src/fdb/subspace"
  "github.com/apple/foundationdb/bindings/go/src/fdb/tuple"
)

const grams = 3
// Key structures:
//   Index for search by the rune:
//     dir, context, "R", runes, order, id, pos
//   Index for clear last search index for the id:
//     dir, context, "I", id, order, rune

func dbAndContextSubspac(dirName string, context string) (fdb.Transactor, subspace.Subspace) {
	// Open the default database from the system cluster
	db := fdb.MustOpenDefault()
	// Directory subspace
	dir, err := directory.CreateOrOpen(db, []string{dirName}, nil)
	if err != nil {
		log.Fatal(err)
	}
	return db, dir.Sub(context)
}

func ClearIndex(dir string, context string, id string) {
	db, contextSubspace := dbAndContextSubspac(dir, context)

	_, err := db.Transact(func (tr fdb.Transaction) (ret interface{}, e error) {
		baseKey := contextSubspace.Sub("I", id)
		ri := tr.GetRange(baseKey, fdb.RangeOptions{}).Iterator()
		for ri.Advance() {
			kv := ri.MustGet()
			t, err := baseKey.Unpack(kv.Key)
			if err != nil {
				log.Fatalf("Uppack failed: %+v", err)
			}
			order := t[0]
			str := t[1]
			tr.ClearRange(contextSubspace.Sub("R", str, order, id))
		}
		tr.ClearRange(baseKey)
		return
	})
	if err != nil {
		log.Fatalf("clearIndex failed (%v)", err)
	}
}

func CreateIndex(dir string, context string, order int64, id string, inputText string) {
	db, contextSubspace := dbAndContextSubspac(dir, context)
	// Clear last index for the id
	ClearIndex(dir, context, id)
	// Create index
	runes := []rune(strings.ToLower(inputText))
	// fmt.Printf("createIndex: id=%+v text='%+v'[%d]\n", id, inputText, len(runes))
	_, err := db.Transact(func (tr fdb.Transaction) (ret interface{}, e error) {
		// for i, w := 0, 0; i < len(text); i+= w {
		for i, _ := range runes {
			n := grams
			if i + n >  len(runes) {
				n = len(runes) - i
			}
			// fmt.Printf("runes[%d:%d]\n", i, n)
			str := string(runes[i:i+n])
			// Create key for search
			// fmt.Printf("  key: str=%+v order=%d, id=%s, pos=%d\n", str, order, id, i)
			key := contextSubspace.Sub("R", str, order, id, i)
			// fmt.Printf("creatKey: %#v\n", fdb.Key(key.Bytes()))
			tr.Set(key, []byte("\x01"))
			// Create key for clear old search key
			tr.Set(contextSubspace.Sub("I", id, order, str), []byte("\x01"))
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
	db, contextSubspace := dbAndContextSubspac(dir, context)

	runes := []rune(strings.ToLower(term))
	runeIndex := 0

	firstRunes := runes
	if len(runes) > grams {
		firstRunes = runes[:grams]
	}
	keyBytes := append(append(contextSubspace.Sub("R").Bytes(), 0x02), []byte(string(firstRunes))...)
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
						nextKey := contextSubspace.Sub("R", str, future.Order, future.Id, future.StartPos + nextRuneIndex)
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
						t, err := contextSubspace.Sub("R").Unpack(kv.Key)
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
				} // rundINdex == 0 else
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
