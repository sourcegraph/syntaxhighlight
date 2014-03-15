package syntaxhighlight

import (
	"bytes"
	"flag"
	"github.com/sourcegraph/annotate"
	"io/ioutil"
	"path/filepath"
	"reflect"
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

func TestAnnotate(t *testing.T) {
	src := []byte(`a:=2`)
	want := []*annotate.Annotation{
		{Start: 0, End: 1, Left: []byte(`<span class="pln">`), Right: []byte("</span>")},
		{Start: 1, End: 2, Left: []byte(`<span class="pun">`), Right: []byte("</span>")},
		{Start: 2, End: 3, Left: []byte(`<span class="pun">`), Right: []byte("</span>")},
		{Start: 3, End: 4, Left: []byte(`<span class="dec">`), Right: []byte("</span>")},
	}
	got, err := Annotate(src, HTMLAnnotator(DefaultHTMLConfig))
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(want, got) {
		t.Errorf("want %+v, got %#v", want, got)
		for _, g := range got {
			t.Logf("%+v  %q  LEFT=%q RIGHT=%q", g, src[g.Start:g.End], g.Left, g.Right)
		}
	}
}
