package main

import (
	"github.com/apple/foundationdb/bindings/go/src/fdb"
  "github.com/apple/foundationdb/bindings/go/src/fdb/directory"
  // "github.com/apple/foundationdb/bindings/go/src/fdb/subspace"
  "github.com/apple/foundationdb/bindings/go/src/fdb/tuple"
	"log"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"
)

func createIndex(partition string, id string, inputText string) {
	// Open the default database from the system cluster
	db := fdb.MustOpenDefault()
	// directory
	dir, err := directory.CreateOrOpen(db, []string{"gyazo"}, nil)
	if err != nil {
		log.Fatal(err)
	}
	ss := dir.Sub(partition)

	// Create index
	// fmt.Printf("Create Indexes\n")
	_, err = db.Transact(func (tr fdb.Transaction) (ret interface{}, e error) {
		// text := strings.ToLower("Windows と macOS")
		// fmt.Printf("  text: %v\n", text)
		text := strings.ToLower(inputText)

		for i, w := 0, 0; i < len(text); i+= w {
			runeValue, width := utf8.DecodeRuneInString(text[i:])
			// key := termSS.Pack(tuple.Tuple{string(runeValue), i})
			// key := ss.Sub(string(runeValue)).Pack(tuple.Tuple{i})
			key := ss.Sub(string(runeValue), id, i)
			// fmt.Printf("%v\n", key)
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

func search(partition string, term string) []string {
	// Open the default database from the system cluster
	db := fdb.MustOpenDefault()
	// directory
	dir, err := directory.CreateOrOpen(db, []string{"gyazo"}, nil)
	if err != nil {
		log.Fatal(err)
	}
	ss := dir.Sub(partition)

	_, err = db.Transact(func (tr fdb.Transaction) (ret interface{}, e error) {
		text := strings.ToLower(term)
		// fmt.Printf("  text: %v\n", text)

		runes := []rune(text)

		key := ss.Sub(string(runes[0]))
		ri := tr.GetRange(key, fdb.RangeOptions{}).Iterator()
		for ri.Advance() {
			// First rune
			fmt.Printf("First rune: %v\n", string(runes[0]))
			kv := ri.MustGet()
			fmt.Printf("%v\n", kv)
			t, err := key.Unpack(kv.Key)
			if err != nil {
				log.Fatalf("Uppack failed")
			}
			pos := t[0].(int64)
			match := true
			// next runes
			for i := 1; i < len(runes); i++ {
				fmt.Printf("i: %v, rune: %v\n", i, string(runes[i]))
				nextKey := ss.Sub(string(runes[i])).Pack(tuple.Tuple{int(pos) + i})
				fmt.Printf("key: %v\n", nextKey)
				v := tr.Get(nextKey).MustGet()
				fmt.Printf("v: %v\n", v)
				if string(v) == "" {
					fmt.Printf("not matched\n")
					match = false
					break
				}
			}
			if match {
				fmt.Printf("matched position: %v\n", pos)
			}
		}
		return
	})
	if err != nil {
	    log.Fatalf("Unable to read FDB database value (%v)", err)
	}
	return []string{}
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

	partition := getParamOrErrorResponse(w, r.Form, "partition")
	if partition == "" {
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

	createIndex(partition, id, content)
	fmt.Fprintf(w, "Index created for partition='%s', id='%s'\n", partition, id)
}

func getSearchHandler(w http.ResponseWriter, r *http.Request) {
	if (r.Method != "GET") {
		w.WriteHeader(http.StatusMethodNotAllowed)
		fmt.Fprintf(w, "Error: GET only.\n")
		return
	}
	q := r.URL.Query()
	log.Printf("GET /search query:%v\n", q)
	partition := getParamOrErrorResponse(w, q, "partition")
	if partition == "" {
		return
	}
	term := getParamOrErrorResponse(w, q, "term")
	if term == "" {
		return
	}

	ids := search(partition, term)
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
