// Package fixtures reads the JSON test fixtures of the repository's fixtures folder for the package tests.
package fixtures

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/csstree/csstree/internal/js"
)

// Test is one fixture test: its name, its key in the file, the folder (scope) it was found in and its JSON
// object.
type Test struct {
	Name  string
	Key   string
	Scope string
	Data  *js.Object
}

// Load reads every *.json file under dir; each key holds a test or an array of tests.
func Load(dir string) ([]Test, error) {
	var tests []Test
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".json" {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		rel = filepath.ToSlash(rel)
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		file, err := js.ParseObject(js.DecodeUTF8(data))
		if err != nil {
			return err
		}
		for _, key := range file.Keys() {
			entries := []any{file.Get(key)}
			if list, ok := file.Get(key).([]any); ok {
				entries = list
			}
			for i, entry := range entries {
				test, ok := entry.(*js.Object)
				if !ok {
					continue
				}
				name := rel + "#" + key
				if len(entries) > 1 {
					name += "#" + strconv.Itoa(i+1)
				}
				tests = append(tests, Test{Name: name, Key: key, Scope: strings.Split(rel, "/")[0], Data: test})
			}
		}
		return nil
	})
	sort.SliceStable(tests, func(i, j int) bool { return tests[i].Name < tests[j].Name })
	return tests, err
}

// WithoutLoc serialises a value as JSON with every loc property left out.
func WithoutLoc(v any) string {
	parsed, err := js.ParseJSON(js.Stringify(v))
	if err != nil {
		return err.Error()
	}
	var strip func(any)
	strip = func(x any) {
		switch o := x.(type) {
		case *js.Object:
			o.Delete("loc")
			for _, k := range o.Keys() {
				strip(o.Get(k))
			}
		case []any:
			for _, e := range o {
				strip(e)
			}
		}
	}
	strip(parsed)
	return js.Stringify(parsed)
}
