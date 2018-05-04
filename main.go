package main

import (
	"github.com/apple/foundationdb/bindings/go/src/fdb"
  "github.com/apple/foundationdb/bindings/go/src/fdb/directory"
  // "github.com/apple/foundationdb/bindings/go/src/fdb/subspace"
  "github.com/apple/foundationdb/bindings/go/src/fdb/tuple"
	"log"
	"fmt"
	"strings"
	"unicode/utf8"
)

// func createIndex(text string) {
	
// }

func main() {
	// Different API versions may expose different runtime behaviors.
	fdb.MustAPIVersion(510)
	// Open the default database from the system cluster
	db := fdb.MustOpenDefault()


	// directory
	dir, err := directory.CreateOrOpen(db, []string{"gyazo"}, nil)
	if err != nil {
		log.Fatal(err)
	}
	// subspace
	termSS := dir.Sub("term")


	// Create index
	fmt.Printf("Create Indexes\n")
	_, err = db.Transact(func (tr fdb.Transaction) (ret interface{}, e error) {
		text := strings.ToLower("Windows と macOS")
		fmt.Printf("  text: %v\n", text)
		// tr.Set(fdb.Key("hello"), []byte{})
		// fmt.Printf("hello: %v\n", tr.Get(fdb.Key("hello")).MustGet())

		for i, w := 0, 0; i < len(text); i+= w {
			runeValue, width := utf8.DecodeRuneInString(text[i:])
			// key := termSS.Pack(tuple.Tuple{string(runeValue), i})
			key := termSS.Sub(string(runeValue)).Pack(tuple.Tuple{i})
			fmt.Printf("%v\n", key)
			tr.Set(key, []byte("\x01"))
			w = width
		}
		return
	})
	if err != nil {
		log.Fatalf("Unable to set FDB database value (%v)", err)
	}

	// Search index
	fmt.Printf("Search Indexes\n")
	_, err = db.Transact(func (tr fdb.Transaction) (ret interface{}, e error) {
		text := strings.ToLower("mac")
		fmt.Printf("  text: %v\n", text)

		runes := []rune(text)

		key := termSS.Sub(string(runes[0]))
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
				nextKey := termSS.Sub(string(runes[i])).Pack(tuple.Tuple{int(pos) + i})
				// ret := tr.Get(nextKey).MustGet()
				// fmt.Printf("ret: %v\n", ret)
				fmt.Printf("key: %v\n", nextKey)
				v := tr.Get(nextKey).MustGet()
				fmt.Printf("v: %v\n", v)
				if string(v) == "" {
					fmt.Printf("not matched\n")
					match = false
					break
				}
				// fmt.Printf("v: %v\n", kv.)
				// if err != nil {
				// 	fmt.Printf("not matched\n")
				// 	match = false
				// 	break
				// }
			}
			if match {
				fmt.Printf("matched position: %v\n", pos)
			}
			// fmt.Printf("%v\n", t)
		}

		// for i, w := 0, 0; i < len(text); i+= w {
		// 	runeValue, width := utf8.DecodeRuneInString(text[i:])
		// 	key := termSS.Sub(string(runeValue))
		// 	ri := tr.GetRange(key, fdb.RangeOptions{}).Iterator()
		// 	for ri.Advance() {
		// 		kv := ri.MustGet()
		// 		// fmt.Printf("%v\n", kv)
		// 		t, err := key.Unpack(kv.Key)
		// 		if err != nil {
		// 			log.Fatalf("Uppack failed")
		// 		}
		// 		// fmt.Printf("%v\n", t)
		// 	}
		// 	// fmt.Printf("%v\n", rangeResults)
		// 	w = width
		// }
		return
	})
	if err != nil {
	    log.Fatalf("Unable to read FDB database value (%v)", err)
	}





	// courseSS := schedulingDir.Sub("class")
	// fmt.Printf("courseSS: %v\n", courseSS)
	// attendSS := schedulingDir.Sub("attends")
	// fmt.Printf("attendSS: %v\n", attendSS)

	// _, err = db.Transact(func (tr fdb.Transaction) (ret interface{}, e error) {
	// 	// tr.Set(fdb.Key("hello2"), []byte("world"))
	// 	key := attendSS.Pack(tuple.Tuple{"hiroshi", "math"})
	// 	fmt.Printf("%v\n", key)
	// 	tr.Set(key, []byte{})
	// 	return
	// })
	// if err != nil {
	// 	log.Fatalf("Unable to set FDB database value (%v)", err)
	// }

	// ret, err := db.Transact(func (tr fdb.Transaction) (ret interface{}, e error) {
	//     ret = tr.Get(fdb.Key("hello")).MustGet()
	//     return
	// })
	// if err != nil {
	//     log.Fatalf("Unable to read FDB database value (%v)", err)
	// }

	// v := ret.([]byte)
	// fmt.Printf("hello, %s\n", string(v))
}
