package main
import (
	"github.com/apple/foundationdb/bindings/go/src/fdb"
  "github.com/apple/foundationdb/bindings/go/src/fdb/directory"
  // "github.com/apple/foundationdb/bindings/go/src/fdb/subspace"
  // "github.com/apple/foundationdb/bindings/go/src/fdb/tuple"
	"fmt"
	"testing"
)

func clearDirectory(t *testing.T, db fdb.Transactor, dirName string) {
	// Directory subspace
	dir, err := directory.CreateOrOpen(db, []string{dirName}, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Transact(func (tr fdb.Transaction) (ret interface{}, e error) {
		tr.ClearRange(dir)
		return
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestClearDirectory(t *testing.T) {
	fdb.MustAPIVersion(510)

	createIndex("-test", "user_1", "doc_1", "日本語の content")
	searchResult := search("-test", "user_1", "content")
	if len(searchResult.Items) == 0 {
		t.Errorf("Precondition failed. searchResult: %v", searchResult)
	}

	db := fdb.MustOpenDefault()
	clearDirectory(t, db, "-test")

	searchResult = search("-test", "user_1", "content")
	if len(searchResult.Items) > 0 {
		t.Errorf("Directory not cleared. searchResult: %v", searchResult)
	}
}

func TestSearch(t *testing.T) {
	// fdb.MustAPIVersion(510)
	// db := fdb.MustOpenDefault()
	// clearDirectory(t, db, "-test")

	// createIndex("app_1", "user_1", "doc_1", "日本語の content")
	// createIndex("app_1", "user_1", "doc_2", "english content")
	// fmt.Printf("search 'content': %v\n", search("app_1", "user_1", "content"))
	// fmt.Printf("search '日本語': %v\n", search("app_1", "user_1", "日本語"))
	// fmt.Printf("search 'unmatch: '%v\n", search("app_1", "user_1", "unmatch"))

	// fmt.Printf("update 'doc_2'\n");
	// createIndex("app_1", "user_1", "doc_2", "english コンテンツ")
	// fmt.Printf("search 'content': %v\n", search("app_1", "user_1", "content"))
	// fmt.Printf("search 'コンテンツ': %v\n", search("app_1", "user_1", "コンテンツ"))

	// fmt.Printf("clear 'doc_1'\n");
	// clearIndex("app_1", "user_1", "doc_1")
	// fmt.Printf("search 'content': %v\n", search("app_1", "user_1", "content"))
}

func TestSearchSample(t *testing.T) {
	fdb.MustAPIVersion(510)
	// db := fdb.MustOpenDefault()

	fmt.Printf("search 'app': %v\n", search("-gyazo", "519effd2dc7d3dd10c185a2e", "アプリ"))
}
