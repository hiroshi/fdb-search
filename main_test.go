package main
import (
	"github.com/apple/foundationdb/bindings/go/src/fdb"
  // "github.com/apple/foundationdb/bindings/go/src/fdb/directory"
  // "github.com/apple/foundationdb/bindings/go/src/fdb/subspace"
  "github.com/apple/foundationdb/bindings/go/src/fdb/tuple"
	"fmt"
	"testing"
)

func TestIndex(t *testing.T) {
	fdb.MustAPIVersion(510)
	db := fdb.MustOpenDefault()
	// dir, err := directory.CreateOrOpen(db, []string{"gyazo"}, nil)
	// if err != nil {
	// 	t.Fatal(err)
	// }
	_, err := db.Transact(func (tr fdb.Transaction) (ret interface{}, e error) {
		// tr.ClearRange(dir)
		tr.ClearRange(tuple.Tuple{})
		return
	})
	if err != nil {
		t.Fatal(err)
	}

	createIndex("user_1", "doc_1", "日本語の content")
	createIndex("user_1", "doc_2", "english content")
	fmt.Printf("%v\n", search("user_1", "content"))
	fmt.Printf("%v\n", search("user_1", "日本語"))
	fmt.Printf("%v\n", search("user_1", "unmatch"))
}
