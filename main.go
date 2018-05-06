package main

import (
	"github.com/apple/foundationdb/bindings/go/src/fdb"
  "github.com/apple/foundationdb/bindings/go/src/fdb/directory"
  // "github.com/apple/foundationdb/bindings/go/src/fdb/subspace"
  // "github.com/apple/foundationdb/bindings/go/src/fdb/tuple"
	"log"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"
)

// func clearIndex(scope string, id string) {
	
// }

func createIndex(scope string, id string, inputText string) {
	// Open the default database from the system cluster
	db := fdb.MustOpenDefault()
	// directory
	dir, err := directory.CreateOrOpen(db, []string{"gyazo"}, nil)
	if err != nil {
		log.Fatal(err)
	}
	scopeSubspace := dir.Sub(scope)

	// Create index
	// fmt.Printf("Create Indexes\n")
	_, err = db.Transact(func (tr fdb.Transaction) (ret interface{}, e error) {
		// text := strings.ToLower("Windows と macOS")
		// fmt.Printf("  text: %v\n", text)
		text := strings.ToLower(inputText)

		for i, w := 0, 0; i < len(text); i+= w {
			r, width := utf8.DecodeRuneInString(text[i:])
			key := scopeSubspace.Sub("i", string(r), id, i)
			tr.Set(key, []byte("\x01"))

			
			w = width
		}
		return
	})
	if err != nil {
		log.Fatalf("Unable to set FDB database value (%v)", err)
	}
}

// type SearchResult struct {
// 	: 
// }

func search(scope string, term string) []string {
	// Open the default database from the system cluster
	db := fdb.MustOpenDefault()
	// directory
	dir, err := directory.CreateOrOpen(db, []string{"gyazo"}, nil)
	if err != nil {
		log.Fatal(err)
	}
	scopeSubspace := dir.Sub(scope)

	results := []string{}

	_, err = db.Transact(func (tr fdb.Transaction) (ret interface{}, e error) {
		text := strings.ToLower(term)
		// fmt.Printf("  text: %v\n", text)

		runes := []rune(text)
		// fmt.Printf("runes: %v\n", runes)
		key := scopeSubspace.Sub("i", string(runes[0]))
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
			id := t[0]
			pos := int(t[1].(int64)) + len(string(runes[0]))
			match := true
			// next runes
			for i := 1; i < len(runes); i++ {
				// fmt.Printf("i: %v, rune: %v\n", i, string(runes[i]))
				// nextKey := scopeSubspace.Sub("i", string(runes[i])).Pack(tuple.Tuple{id, int(pos) + i})
				nextKey := scopeSubspace.Sub("i", string(runes[i]), id, pos)
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
				results = append(results, id.(string)) // maybe inefficient
			}
		}
		return
	})
	if err != nil {
	    log.Fatalf("Unable to read FDB database value (%v)", err)
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

	scope := getParamOrErrorResponse(w, r.Form, "scope")
	if scope == "" {
		return
	}
	id := getParamOrErrorResponse(w, r.Form, "id")
	if id == "" {
		return
	}
	content := getParamOrErrorResponse(w, r.Form, "content")
	if content == "" {
		return
	}

	createIndex(scope, id, content)
	fmt.Fprintf(w, "Index created for scope='%s', id='%s'\n", scope, id)
}

func getSearchHandler(w http.ResponseWriter, r *http.Request) {
	if (r.Method != "GET") {
		w.WriteHeader(http.StatusMethodNotAllowed)
		fmt.Fprintf(w, "Error: GET only.\n")
		return
	}
	q := r.URL.Query()
	log.Printf("GET /search query:%v\n", q)
	scope := getParamOrErrorResponse(w, q, "scope")
	if scope == "" {
		return
	}
	term := getParamOrErrorResponse(w, q, "term")
	if term == "" {
		return
	}

	ids := search(scope, term)
	fmt.Fprintf(w, "Found: %v\n", ids)
}

func main() {
	// Different API versions may expose different runtime behaviors.
	fdb.MustAPIVersion(510)

	// createIndex("Windows と macOS")
	// search("mac")

	http.HandleFunc("/index", postIndexHandler)
	http.HandleFunc("/search", getSearchHandler)
	addr := ":12345"
	log.Printf("Starging server: %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
