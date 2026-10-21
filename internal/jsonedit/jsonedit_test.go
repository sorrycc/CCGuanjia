package jsonedit

import (
	"bytes"
	"encoding/json"
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

func TestPaths(t *testing.T) {
	src := []byte(`{"b": 1, "permissions": {"deny": [], "allow": ["Bash(ls)"]}, "a": 2}`)
	o, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	var allow []string
	if ok, err := o.GetIn([]string{"permissions", "allow"}, &allow); !ok || err != nil || !slices.Equal(allow, []string{"Bash(ls)"}) {
		t.Fatalf("GetIn: %v %v %v", ok, err, allow)
	}
	c := o.Clone()
	if err := c.SetIn([]string{"permissions", "allow"}, []string{"Read"}); err != nil {
		t.Fatal(err)
	}
	if err := c.SetIn([]string{"env", "X"}, "1"); err != nil {
		t.Fatal(err)
	}
	if !c.DeleteIn([]string{"permissions", "deny"}) || c.DeleteIn([]string{"permissions", "ask"}) || c.DeleteIn([]string{"none", "x"}) {
		t.Fatal("DeleteIn")
	}
	if err := c.SetIn([]string{"b", "x"}, 1); err == nil {
		t.Error("set inside a number")
	}
	if got, want := compact(t, c), `{"b":1,"permissions":{"allow":["Read"]},"a":2,"env":{"X":"1"}}`; got != want {
		t.Errorf("got %s want %s", got, want)
	}
	if got := compact(t, o); got != `{"b":1,"permissions":{"deny":[],"allow":["Bash(ls)"]},"a":2}` {
		t.Errorf("the original changed: %s", got)
	}
}

func compact(t *testing.T, o *Object) string {
	t.Helper()
	out, err := o.Marshal("")
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	json.Compact(&b, out)
	return b.String()
}
