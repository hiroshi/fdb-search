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
			tr.Set(contextSubspace.Sub("R", str, order, id, i), []byte("\x01"))
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

// func forward() () {
// 		if runeIndex + grams < len(runes) {
// 			str := string(runes[nextRuneIndex : nextRuneIndex + grams])
// 			pos := future.StartPos + nextRuneIndex
// 			nextKey := contextSubspace.Sub("R", str, future.Order, future.Id, pos)
// 			nextFutures = append(nextFutures, SearchFuture{future.Order, future.Id, future.StartPos, nextRuneIndex, tr.Get(nextKey)})
// 		} else {
// 			item := SearchResultItem{future.Id.(string), future.StartPos}
// 			items = append(items, item)
// 			lastMatchId = future.Id.(string)
// 		}
// }

func Search(dir string, context string, term string) SearchResult {
	db, contextSubspace := dbAndContextSubspac(dir, context)

	runes := []rune(strings.ToLower(term))
	runeIndex := 0

	n := grams
	if len(runes) < n {
		n = len(runes)
	}
	firstString := string(runes[0:n])
	searchKey := contextSubspace.Sub("R", firstString)
	beginKey := searchKey
	endKey := contextSubspace.Sub("R", firstString + "0xFF")

	futures := []SearchFuture{}
	nextFutures := []SearchFuture{}

	items := []SearchResultItem{}
	lastMatchId := ""

	rangeContinue := true
	for rangeContinue {
		// NOTE: ruens and futures are shifted as processed to be able to contine on transaction retry
		_, err := db.ReadTransact(func (tr fdb.ReadTransaction) (ret interface{}, e error) {
			// fmt.Printf("%v < %v\n", runeIndex, len(runes))
			for {
				// fmt.Printf("%v < %v\n", runeIndex, len(runes))
				nextRuneIndex := runeIndex + grams
				if runeIndex + grams < len(runes) && len(runes) < runeIndex + grams * 2 {
					nextRuneIndex = len(runes) - grams
				}

				if runeIndex == 0 {
					keyRange := fdb.KeyRange{beginKey, endKey}
					ri := tr.GetRange(keyRange, fdb.RangeOptions{Reverse: true}).Iterator()
					// Iterate through keys for the first rune to get all future of keys for the second rune
					for rangeContinue {
						if len(futures) > 10000 {
							break
						}
						rangeContinue = ri.Advance()
						if !rangeContinue {
							break
						}
						kv := ri.MustGet()
						endKey = subspace.FromBytes(kv.Key)
						t, err := contextSubspace.Sub("R").Unpack(kv.Key)
						if err != nil {
							log.Fatalf("Unpack failed: %+v.", err)
						}
						_ = t[0].(string)
						order := t[1].(int64)
						id := t[2]
						startPos := int(t[3].(int64))
						pos := startPos + nextRuneIndex
						if len(runes) > grams {
							str := string(runes[nextRuneIndex : nextRuneIndex + grams])
							nextKey := contextSubspace.Sub("R", str, order, id, pos)
							future := SearchFuture{order, id, startPos, nextRuneIndex, tr.Get(nextKey)}
							futures = append(futures, future)
						} else {
							if lastMatchId == id {
								continue
							}
							item := SearchResultItem{id.(string), startPos}
							items = append(items, item)
							lastMatchId = id.(string)
						}
					}
					runeIndex = nextRuneIndex
				} else {
					nextFutures = futures[:0]

					for len(futures) > 0 {
						future := futures[0]
						// Skip duplicated Id from result
						if lastMatchId != future.Id {
							v := future.Future.MustGet()
							if string(v) != "" {
								if runeIndex + grams < len(runes) {
									str := string(runes[nextRuneIndex : nextRuneIndex + grams])
									pos := future.StartPos + nextRuneIndex
									nextKey := contextSubspace.Sub("R", str, future.Order, future.Id, pos)
									nextFutures = append(nextFutures, SearchFuture{future.Order, future.Id, future.StartPos, nextRuneIndex, tr.Get(nextKey)})
								} else {
									item := SearchResultItem{future.Id.(string), future.StartPos}
									items = append(items, item)
									lastMatchId = future.Id.(string)
								}
							}
						}
						futures = futures[1:]
					}
					futures = nextFutures
					if runeIndex + grams >= len(runes) {
						break
					}
					runeIndex = nextRuneIndex
				}
			}
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
