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
	"strings"
	"unicode/utf8"
)

func dbAndScopeSubspac(dirName string, scope string) (fdb.Transactor, subspace.Subspace) {
	// Open the default database from the system cluster
	db := fdb.MustOpenDefault()
	// Directory subspace
	dir, err := directory.CreateOrOpen(db, []string{dirName}, nil)
	if err != nil {
		log.Fatal(err)
	}
	return db, dir.Sub(scope)
}

func clearIndex(dir string, scope string, doc string) {
	db, scopeSubspace := dbAndScopeSubspac(dir, scope)

	_, err := db.Transact(func (tr fdb.Transaction) (ret interface{}, e error) {
		baseKey := scopeSubspace.Sub("D", doc)
		ri := tr.GetRange(baseKey, fdb.RangeOptions{}).Iterator()
		for ri.Advance() {
			kv := ri.MustGet()
			t, err := baseKey.Unpack(kv.Key)
			if err != nil {
				log.Fatalf("Uppack failed")
			}
			tr.ClearRange(scopeSubspace.Sub("R", t[0], doc))
		}
		tr.ClearRange(baseKey)
		return
	})
	if err != nil {
		log.Fatalf("clearIndex failed (%v)", err)
	}
}

func createIndex(dir string, scope string, doc string, inputText string) {
	db, scopeSubspace := dbAndScopeSubspac(dir, scope)
	// Clear last index
	clearIndex(dir, scope, doc)
	// Create index
	// fmt.Printf("Create Indexes\n")
	_, err := db.Transact(func (tr fdb.Transaction) (ret interface{}, e error) {
		// text := strings.ToLower("Windows と macOS")
		// fmt.Printf("  text: %v\n", text)
		text := strings.ToLower(inputText)

		for i, w := 0, 0; i < len(text); i+= w {
			r, width := utf8.DecodeRuneInString(text[i:])
			// Create key for search
			tr.Set(scopeSubspace.Sub("R", string(r), doc, i), []byte("\x01"))
			// Create key for clear old search key
			tr.Set(scopeSubspace.Sub("D", doc, string(r)), []byte("\x01"))

			w = width
		}
		return
	})
	if err != nil {
		log.Fatalf("createIndex failed (%v)", err)
	}
}

type SearchResultItem struct {
	Doc string `json:"doc"`
	Pos int `json:"pos"`
}

type SearchResult struct {
	Items []SearchResultItem `json:"items"`
	Count int `json:"count"`
}

type SearchFuture struct {
	// Key fdb.Key
  Doc tuple.TupleElement
	StartPos int
  Pos int
	Future fdb.FutureByteSlice
}

func search(dir string, scope string, term string) SearchResult {
	db, scopeSubspace := dbAndScopeSubspac(dir, scope)

	runes := []rune(strings.ToLower(term))
	searchKey := scopeSubspace.Sub("R", string(runes[0]))
	beginKey := searchKey
	endKey := scopeSubspace.Sub("R", string(runes[0]) + "0xFF")

	items := []SearchResultItem{}

	_, err := db.Transact(func (tr fdb.Transaction) (ret interface{}, e error) {
		futures := []SearchFuture{}

		// Get an iterator for the first rune
		keyRange := fdb.KeyRange{beginKey, endKey}
		ri := tr.GetRange(keyRange, fdb.RangeOptions{}).Iterator()
		// Iterate through keys for the first rune to get all future of keys for the second rune
		for ri.Advance() {
			kv := ri.MustGet()
			beginKey = subspace.FromBytes(kv.Key)
			t, err := searchKey.Unpack(kv.Key)
			if err != nil {
				log.Fatalf("Uppack failed")
			}
			doc := t[0]
			startPos := int(t[1].(int64))
			pos := startPos + len(string(runes[0]))

			nextKey := scopeSubspace.Sub("R", string(runes[1]), doc, pos)
			pos +=  len(string(runes[1]))
			futures = append(futures, SearchFuture{doc, startPos, pos, tr.Get(nextKey)})
		}
		// Check the second value of futures
		for i := 2; i < len(runes); i++ {
			nextFutures := futures[:0]
			for _, future := range futures {
				v := future.Future.MustGet()
				if string(v) != "" {
					if i + 1 < len(runes) {
						nextKey := scopeSubspace.Sub("R", string(runes[i]), future.Doc, future.Pos)
						pos := future.Pos + len(string(runes[i]))
						nextFutures = append(nextFutures, SearchFuture{future.Doc, future.StartPos, pos, tr.Get(nextKey)})
					} else {
						item := SearchResultItem{future.Doc.(string), future.StartPos}
						items = append(items, item)
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
	scope := getParamOrErrorResponse(w, r.Form, "scope")
	if scope == "" {
		return
	}
	doc := getParamOrErrorResponse(w, r.Form, "doc")
	if doc == "" {
		return
	}
	content := getParamOrErrorResponse(w, r.Form, "content")
	if content == "" {
		return
	}

	createIndex(dir, scope, doc, content)
	fmt.Fprintf(w, "Index created for scope='%s', doc='%s'\n", scope, doc)
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
	scope := getParamOrErrorResponse(w, q, "scope")
	if scope == "" {
		return
	}
	term := getParamOrErrorResponse(w, q, "term")
	if term == "" {
		return
	}

	result := search(dir, scope, term)
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
