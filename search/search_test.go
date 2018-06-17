package search
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

	CreateIndex("-test", "user_1", 0, "doc_1", "日本語の content")
	searchResult := Search("-test", "user_1", "content")
	if len(searchResult.Items) == 0 {
		t.Errorf("Precondition failed. searchResult: %+v", searchResult)
	}

	db := fdb.MustOpenDefault()
	clearDirectory(t, db, "-test")

	searchResult = Search("-test", "user_1", "content")
	if len(searchResult.Items) > 0 {
		t.Errorf("Directory not cleared. searchResult: %+v", searchResult)
	}
}

func TestSearch(t *testing.T) {
	fdb.MustAPIVersion(510)
	db := fdb.MustOpenDefault()

	t.Run("single character", func(t *testing.T) {
		clearDirectory(t, db, "-test")

		CreateIndex("-test", "user_1", 0, "1", "test")

		result := Search("-test", "user_1", "t")
		if result.Count != 1 {
			t.Errorf("result.Count must be 1. result: %+v", result)
		}
	})

	t.Run("more than 6 characters", func(t *testing.T) {
		clearDirectory(t, db, "-test")

		CreateIndex("-test", "user_1", 0, "1", "test content.")

		result := Search("-test", "user_1", "content")
		if result.Count != 1 {
			t.Errorf("result.Count must be 1. result: %+v", result)
		}
	})

	t.Run("updated content", func(t *testing.T) {
		clearDirectory(t, db, "-test")

		CreateIndex("-test", "user_1", 0, "1", "id:1 の最初の text")
		CreateIndex("-test", "user_1", 2, "1", "id:1 の更新した text")

		result := Search("-test", "user_1", "最初")
		if result.Count != 0 {
			t.Errorf("Old term should not be found. result: %+v", result)
		}
		result = Search("-test", "user_1", "更新した")
		if result.Count != 1 || result.Items[0].Id != "1" {
			t.Errorf("New term should be found. result: %+v", result)
		}
	})

	t.Run("Multiple term in a context of single id", func(t *testing.T) {
		clearDirectory(t, db, "-test")

		CreateIndex("-test", "user_1", 0, "4", "duplicated duplicated")

		result := Search("-test", "user_1", "dup")
		if result.Count != 1 {
			t.Errorf("result.Items[].Id must be unique. result: %+v", result)
		}
	})

	t.Run("Multiple items in a result", func(t *testing.T) {
		clearDirectory(t, db, "-test")

		CreateIndex("-test", "user_1", 0, "4", "duplicated duplicated")
		CreateIndex("-test", "user_1", 0, "5", "not duplicated")

		result := Search("-test", "user_1", "dup")
		if result.Count != 2 {
			t.Errorf("result.Items[].Id must be unique. result: %+v", result)
		}
	})
}
