package main
import (
	"github.com/apple/foundationdb/bindings/go/src/fdb"
  "github.com/apple/foundationdb/bindings/go/src/fdb/directory"
  // "github.com/apple/foundationdb/bindings/go/src/fdb/subspace"
  // "github.com/apple/foundationdb/bindings/go/src/fdb/tuple"
	// "fmt"
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

	createIndex("-test", "user_1", 0, "doc_1", "日本語の content")
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
	fdb.MustAPIVersion(510)
	db := fdb.MustOpenDefault()
	clearDirectory(t, db, "-test")

	createIndex("-test", "user_1", 0, "1", "id:1 の最初の text")
	createIndex("-test", "user_1", 2, "1", "id:1 の更新した text")
	createIndex("-test", "user_1", 1, "2", "text contenxt of id:2")
	createIndex("-test", "user_1", 3, "3", "text of id:3")
	createIndex("-test", "user_2", 3, "a", "text contenxt for user_2")

	// Search deleted old term
	result := search("-test", "user_1", "最初")
	if result.Count > 0 {
		t.Errorf("Old term should not be found. result: %+v", result)
	}
	// Search new term
	result = search("-test", "user_1", "更新")
	if result.Count == 0 || result.Items[0].Id != "1" {
		t.Errorf("New term should be found. result: %+v", result)
	}
	// Search result order
	result = search("-test", "user_1", "text")
	if result.Count != 3 || result.Items[0].Id != "3" || result.Items[1].Id != "1" || result.Items[2].Id != "2" {
	 	t.Errorf("result.Items sholuld be in order of id [3,1,2]. result: %+v", result)
	}
	// Unique id in a result
	createIndex("-test", "user_1", 0, "4", "duplicated duplicated")
	createIndex("-test", "user_1", 0, "5", "not duplicated")
	result = search("-test", "user_1", "dup")
	if result.Count != 2 {
	 	t.Errorf("result.Items[].Id must be unique. result: %+v", result)
	}

}

// func TestSampleSearch(t *testing.T) {
// 	fdb.MustAPIVersion(510)
// 	// db := fdb.MustOpenDefault()

// 	fmt.Printf("search 'app': %v\n", search("-gyazo", "519effd2dc7d3dd10c185a2e", "アプリ"))
// }
