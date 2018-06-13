package main

import (
	"encoding/json"
	"github.com/apple/foundationdb/bindings/go/src/fdb"
  "github.com/apple/foundationdb/bindings/go/src/fdb/directory"
  "github.com/apple/foundationdb/bindings/go/src/fdb/subspace"
  "github.com/apple/foundationdb/bindings/go/src/fdb/tuple"
	"os"
	"log"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	// "unicode/utf8"
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

func clearIndex(dir string, context string, id string) {
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

func createIndex(dir string, context string, order int64, id string, inputText string) {
	db, contextSubspace := dbAndContextSubspac(dir, context)
	// Clear last index for the id
	clearIndex(dir, context, id)
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
	// Future fdb.FutureKey
}

func search(dir string, context string, term string) SearchResult {
	// fmt.Printf("search: term='%+v'\n", term)
	db, contextSubspace := dbAndContextSubspac(dir, context)

	runes := []rune(strings.ToLower(term))
	// Keep status out of transaction to be able to contine on retry
	runeIndex := 0

	n := grams
	if len(runes) < n {
		n = len(runes)
	}
	firstString := string(runes[0:n])
	// fmt.Printf("firstString: %+v\n", firstString)
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
			// Get an iterator for the first rune
			// fmt.Printf("transaction: len(futures)=%+v\n", len(futures));
			// fmt.Printf("transaction: len(items)=%+v\n", len(items));

			// Refresh futures of old transaction
			// for _, future := range futures {
			// 	pos := future.StartPos + len(string(runes[:runeIndex]))
			// 	key := contextSubspace.Sub("R", string(runes[future.RuneIndex]), future.Order, future.Id, pos)
			// 	future.Future = tr.Get(key)
			// }

			if runeIndex == 0 {
				nextRuneIndex := grams
				if len(runes) < grams * 2 {
					nextRuneIndex = len(runes) - grams
				}

				keyRange := fdb.KeyRange{beginKey, endKey}
				// fmt.Printf("transaction: runes=%+v, range=%+v\n", runes, keyRange);
				// fmt.Printf("beginKey=%+v\n", beginKey)
				// fmt.Printf("keyRange=%+v\n", keyRange)
				ri := tr.GetRange(keyRange, fdb.RangeOptions{Reverse: true}).Iterator()
				// Iterate through keys for the first rune to get all future of keys for the second rune
				for rangeContinue {
					if len(futures) > 10000 {
						break
					}
					rangeContinue = ri.Advance()
					// fmt.Printf("rangeContinue:%+v\n", rangeContinue)
					if !rangeContinue {
						break
					}

					kv := ri.MustGet()
					endKey = subspace.FromBytes(kv.Key)
					// fmt.Printf("beginKey: %+v\n", beginKey)
					t, err := contextSubspace.Sub("R").Unpack(kv.Key)
					if err != nil {
						log.Fatalf("Unpack failed: %+v.", err)
					}
					_ = t[0].(string)
					order := t[1].(int64)
					id := t[2]
					startPos := int(t[3].(int64))
					pos := startPos + nextRuneIndex
					// fmt.Printf("startPos: %+v pos:%+v\n", startPos, pos)
					if len(runes) > grams {
						str := string(runes[nextRuneIndex : nextRuneIndex + grams])
						// fmt.Printf("nextKey: runes[%d:%d], str='%s' order=%d id=%s, pos=%d\n", nextRuneIndex, nextRuneIndex + grams, str, order, id, pos)
						nextKey := contextSubspace.Sub("R", str, order, id, pos)
						future := SearchFuture{order, id, startPos, nextRuneIndex, tr.Get(nextKey)}
						// nextKey := contextSubspace.Sub("R", str)
						// future := SearchFuture{order, id, startPos, 1, tr.GetKey(fdb.FirstGreaterThan(nextKey))}
						// fmt.Printf("future: %+v\n", future)
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
				// fmt.Printf("len(futures)=%+v\n", len(futures))
				runeIndex = nextRuneIndex
			}
			//runes = runes[1:]
			// fmt.Printf("runes=%+v\n", runes);
			// Check the second value of futures
			// for i := 2; i <= len(runes); i++ {
			// for len(runes) > 0 {
			for runeIndex < len(runes) {
				// fmt.Printf("runeIndex=%+v\n", runeIndex);
				nextFutures = futures[:0]
				for len(futures) > 0 {
					future := futures[0]
					// Skip duplicated Id from result
					if lastMatchId != future.Id {
						v := future.Future.MustGet()
						// fmt.Printf("future: %+v => %+v\n", future, v)
						if string(v) != "" {
							if runeIndex + grams < len(runes) {
								// fmt.Printf("runes=%s, len(string[runes[:%d]))=%d\n", string(runes), runeIndex + 1, len(string(runes[:runeIndex + 1])))
								pos := future.StartPos + len(string(runes[:runeIndex + 1]))
								nextKey := contextSubspace.Sub("R", string(runes[runeIndex + 1]), future.Order, future.Id, pos)
								nextFutures = append(nextFutures, SearchFuture{future.Order, future.Id, future.StartPos, runeIndex + 1, tr.Get(nextKey)})
								// nextKey := contextSubspace.Sub("R", string(runes[runeIndex + 1]), future.Order, future.Id, pos)
								// nextFutures = append(nextFutures, SearchFuture{future.Order, future.Id, future.StartPos, runeIndex + 1, tr.Get(nextKey)})
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
				// runes = runes[1:]
				runeIndex++
			}
			// fmt.Printf("len(items): %+v\n", len(items))
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

func getParamOrErrorResponse(w http.ResponseWriter, params map[string][]string, name string) string {
	if len(params[name]) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Error: No '%s' param specified.\n", name)
		return ""
	}
	return params[name][0]
}


func postIndexHandler(w http.ResponseWriter, r *http.Request) {
	if (r.Method != "POST") {
		w.WriteHeader(http.StatusMethodNotAllowed)
		fmt.Fprintf(w, "Error: POST only.\n")
		return
	}
	// r.ParseForm()
	r.ParseMultipartForm(1024*1024)
	log.Printf("POST /index form:%v\n", r.Form)

	dir := getParamOrErrorResponse(w, r.Form, "dir")
	if dir == "" {
		return
	}
	context := getParamOrErrorResponse(w, r.Form, "context")
	if context == "" {
		return
	}
	orderString := getParamOrErrorResponse(w, r.Form, "order")
	if orderString == "" {
		return
	}
	id := getParamOrErrorResponse(w, r.Form, "id")
	if id == "" {
		return
	}
	text := getParamOrErrorResponse(w, r.Form, "text")
	if text == "" {
		return
	}

	order, error := strconv.Atoi(orderString)
	if error != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Error: order must be integer: order=%v, error=%v", orderString, error)
		return
	}

	createIndex(dir, context, int64(order), id, text)
	fmt.Fprintf(w, "Index created for context='%s', id='%s'\n", context, id)
}

func getSearchHandler(w http.ResponseWriter, r *http.Request) {
	if (r.Method != "GET") {
		w.WriteHeader(http.StatusMethodNotAllowed)
		fmt.Fprintf(w, "Error: GET only.\n")
		return
	}
	q := r.URL.Query()
	log.Printf("GET /search query:%v\n", q)
	dir := getParamOrErrorResponse(w, q, "dir")
	if dir == "" {
		return
	}
	context := getParamOrErrorResponse(w, q, "context")
	if context == "" {
		return
	}
	term := getParamOrErrorResponse(w, q, "term")
	if term == "" {
		return
	}

	result := search(dir, context, term)
	resultJson, err := json.Marshal(result)
	if err != nil {
		log.Fatalf("json.Marshal(SearchResult) failed: %v", err)
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, string(resultJson))
}

func main() {
	// Different API versions may expose different runtime behaviors.
	fdb.MustAPIVersion(510)

	// createIndex("Windows と macOS")
	// search("mac")

	http.HandleFunc("/index", postIndexHandler)
	http.HandleFunc("/search", getSearchHandler)
	port := os.Getenv("PORT")
	if port == "" {
		port = "1234"
	}
	addr := ":" + port
	log.Printf("Starging server: %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
