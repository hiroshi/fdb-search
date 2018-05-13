package main

import (
	"github.com/apple/foundationdb/bindings/go/src/fdb"
  "github.com/apple/foundationdb/bindings/go/src/fdb/directory"
  "github.com/apple/foundationdb/bindings/go/src/fdb/subspace"
  // "github.com/apple/foundationdb/bindings/go/src/fdb/tuple"
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

// type SearchResult struct {
// 	: 
// }

func search(dir string, scope string, term string) []string {
	db, scopeSubspace := dbAndScopeSubspac(dir, scope)

	results := []string{}

	_, err := db.Transact(func (tr fdb.Transaction) (ret interface{}, e error) {
		text := strings.ToLower(term)
		// fmt.Printf("  text: %v\n", text)

		runes := []rune(text)
		// fmt.Printf("runes: %v\n", runes)
		key := scopeSubspace.Sub("R", string(runes[0]))
		ri := tr.GetRange(key, fdb.RangeOptions{}).Iterator()
		for ri.Advance() {
			// First rune
			// fmt.Printf("First rune: %v\n", string(runes[0]))
			kv := ri.MustGet()
			// fmt.Printf("kv: %v\n", kv)
			t, err := key.Unpack(kv.Key)
			// fmt.Printf("t: %v\n", t)
			if err != nil {
				log.Fatalf("Uppack failed")
			}
			doc := t[0]
			pos := int(t[1].(int64)) + len(string(runes[0]))
			match := true
			// next runes
			for i := 1; i < len(runes); i++ {
				// fmt.Printf("i: %v, rune: %v\n", i, string(runes[i]))
				// nextKey := scopeSubspace.Sub("R", string(runes[i])).Pack(tuple.Tuple{doc, int(pos) + i})
				nextKey := scopeSubspace.Sub("R", string(runes[i]), doc, pos)
				// fmt.Printf("key: %v\n", nextKey)
				v := tr.Get(nextKey).MustGet()
				// fmt.Printf("v: %v\n", v)
				if string(v) == "" {
					// fmt.Printf("not matched\n")
					match = false
					break
				}
				pos += len(string(runes[i]))
			}
			if match {
				// fmt.Printf("matched position: %v\n", pos)
				results = append(results, doc.(string)) // maybe inefficient
			}
		}
		return
	})
	if err != nil {
	    log.Fatalf("search failed (%v)", err)
	}
	return results
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

	docs := search(dir, scope, term)
	fmt.Fprintf(w, "Found: %v\n", docs)
}

func main() {
	// Different API versions may expose different runtime behaviors.
	fdb.MustAPIVersion(510)

	// createIndex("Windows と macOS")
	// search("mac")

	http.HandleFunc("/index", postIndexHandler)
	http.HandleFunc("/search", getSearchHandler)
	addr := ":1234"
	log.Printf("Starging server: %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
