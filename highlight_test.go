package syntaxhighlight

import (
	"bytes"
	"flag"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"
)

var saveExp = flag.Bool("exp", false, "overwrite all expected output files with actual output (returning a failure)")
var match = flag.String("m", "", "only run tests whose name contains this string")

func TestAsHTML(t *testing.T) {
	dir := "testdata"
	tests, err := ioutil.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range tests {
		name := test.Name()
		if !strings.Contains(name, *match) {
			continue
		}
		if strings.HasSuffix(name, ".html") {
			continue
		}
		path := filepath.Join(dir, name)
		input, err := ioutil.ReadFile(path)
		if err != nil {
			t.Fatal(err)
			continue
		}

		got, err := AsHTML(input)
		if err != nil {
			t.Errorf("%s: AsHTML: %s", name, err)
			continue
		}

		expPath := path + ".html"
		if *saveExp {
			err = ioutil.WriteFile(expPath, got, 0700)
			if err != nil {
				t.Fatal(err)
			}
			continue
		}

		want, err := ioutil.ReadFile(expPath)
		if err != nil {
			t.Fatal(err)
		}

		want = bytes.TrimSpace(want)
		got = bytes.TrimSpace(got)

		if !bytes.Equal(want, got) {
			t.Errorf("%s: want %q, got %q", name, want, got)
			continue
		}
	}

	if *saveExp {
		t.Fatal("overwrote all expected output files with actual output (run tests again without -exp)")
	}
}
