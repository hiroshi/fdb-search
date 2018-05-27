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
	"unicode/utf8"
)

// Key structures:
//   Index for search by the rune:
//     dir, context, "R", rune, order, id, pos
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
				log.Fatalf("Uppack failed")
			}
			order := t[0]
			rune := t[1]
			tr.ClearRange(contextSubspace.Sub("R", rune, order, id))
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
	_, err := db.Transact(func (tr fdb.Transaction) (ret interface{}, e error) {
		text := strings.ToLower(inputText)

		for i, w := 0, 0; i < len(text); i+= w {
			r, width := utf8.DecodeRuneInString(text[i:])
			// Create key for search
			tr.Set(contextSubspace.Sub("R", string(r), order, id, i), []byte("\x01"))
			// Create key for clear old search key
			tr.Set(contextSubspace.Sub("I", id, order, string(r)), []byte("\x01"))

			w = width
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
  Pos int
	Future fdb.FutureByteSlice
}

func search(dir string, context string, term string) SearchResult {
	db, contextSubspace := dbAndContextSubspac(dir, context)

	runes := []rune(strings.ToLower(term))
	searchKey := contextSubspace.Sub("R", string(runes[0]))
	beginKey := searchKey
	endKey := contextSubspace.Sub("R", string(runes[0]) + "0xFF")

	items := []SearchResultItem{}
	lastMatchId := ""

	_, err := db.ReadTransact(func (tr fdb.ReadTransaction) (ret interface{}, e error) {
		// fmt.Printf("transaction: %+v\n", tr)
		futures := []SearchFuture{}

		// Get an iterator for the first rune
		keyRange := fdb.KeyRange{beginKey, endKey}
		ri := tr.GetRange(keyRange, fdb.RangeOptions{Reverse: true}).Iterator()
		// Iterate through keys for the first rune to get all future of keys for the second rune
		for ri.Advance() {
			kv := ri.MustGet()
			beginKey = subspace.FromBytes(kv.Key)
			t, err := searchKey.Unpack(kv.Key)
			if err != nil {
				log.Fatalf("Uppack failed")
			}
			order := t[0].(int64)
			id := t[1]
			startPos := int(t[2].(int64))
			pos := startPos + len(string(runes[0]))

			if len(runes) > 1 {
				nextKey := contextSubspace.Sub("R", string(runes[1]), order, id, pos)
				pos +=  len(string(runes[1]))
				futures = append(futures, SearchFuture{order, id, startPos, pos, tr.Get(nextKey)})
			} else {
				if lastMatchId == id {
					continue
				}
				item := SearchResultItem{id.(string), startPos}
				items = append(items, item)
				lastMatchId = id.(string)
			}
		}
		// Check the second value of futures
		for i := 2; i <= len(runes); i++ {
			nextFutures := futures[:0]
			for _, future := range futures {
				// Skip duplicated Id from result
				if lastMatchId == future.Id {
					continue
				}
				v := future.Future.MustGet()
				if string(v) != "" {
					if i + 1 < len(runes) {
						nextKey := contextSubspace.Sub("R", string(runes[i]), future.Order, future.Id, future.Pos)
						pos := future.Pos + len(string(runes[i]))
						nextFutures = append(nextFutures, SearchFuture{future.Order, future.Id, future.StartPos, pos, tr.Get(nextKey)})
					} else {
						item := SearchResultItem{future.Id.(string), future.StartPos}
						items = append(items, item)
						lastMatchId = future.Id.(string)
					}
				}
			}
			futures = nextFutures
		}
		return
	})
	if err != nil {
	    log.Fatalf("search failed (%v)", err)
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
		fmt.Fprintf(w, "Error: order must be integer: order=%v, error=", orderString, error)
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
		log.Fatal("json.Marshal(SearchResult) failed: %v", err)
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
