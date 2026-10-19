package jsonedit

import (
	"slices"
	"testing"
)

func TestKeepsOrderAndValues(t *testing.T) {
	src := []byte("{\n    \"zeta\": 1,\n    \"alpha\": {\"b\": 2, \"a\": [1, 2]},\n    \"url\": \"a<b>&c\"\n}\n")
	o, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	if got := o.Keys(); !slices.Equal(got, []string{"zeta", "alpha", "url"}) {
		t.Fatalf("keys %v", got)
	}
	if err := o.Set("zeta", 2); err != nil {
		t.Fatal(err)
	}
	o.Set("new", true)
	if !o.Delete("alpha") || o.Delete("missing") {
		t.Fatal("Delete")
	}
	out, err := o.Marshal(Indent(src))
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n    \"zeta\": 2,\n    \"url\": \"a<b>&c\",\n    \"new\": true\n}"
	if string(out) != want {
		t.Errorf("got\n%s\nwant\n%s", out, want)
	}
}

func TestNotAnObject(t *testing.T) {
	for _, s := range []string{"[]", "", "{", `{"a":}`} {
		if _, err := Parse([]byte(s)); err == nil {
			t.Errorf("%q parsed", s)
		}
	}
}

func TestIndent(t *testing.T) {
	if got := Indent([]byte("{\n\t\"a\": 1\n}")); got != "\t" {
		t.Errorf("tab: %q", got)
	}
	if got := Indent([]byte(`{"a":1}`)); got != "  " {
		t.Errorf("default: %q", got)
	}
}
