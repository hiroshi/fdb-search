package main

import (
	"encoding/json"
	"os"
	"log"
	"fmt"
	"net/http"
	"strconv"
	// "strings"

	"github.com/apple/foundationdb/bindings/go/src/fdb"
  // "github.com/apple/foundationdb/bindings/go/src/fdb/directory"
  // "github.com/apple/foundationdb/bindings/go/src/fdb/subspace"
  // "github.com/apple/foundationdb/bindings/go/src/fdb/tuple"

	"github.com/hiroshi/fdb-search/search"
)

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

	search.CreateIndex(dir, context, int64(order), id, text)
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

	result := search.Search(dir, context, term)
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
