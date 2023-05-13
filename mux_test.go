package ich

import (
	"net/http"
	"testing"
)

func TestPathTo(t *testing.T) {
	r := New()
	r.Get("/foo/{bar:[a-z-]+}/*", http.NotFound).Name("foo")
	r.Route("/nested/{foo}", func(r *Mux) {
		r.Get("/bar/{baz}", http.NotFound).Name("bar")
	})

	tests := []struct {
		name string
		args []string
		path string
	}{
		{
			"foo",
			[]string{"bar", "value", "*", "wild/card"},
			"/foo/value/wild/card",
		},
		{
			"bar",
			[]string{"foo", "value1", "baz", "value2"},
			"/nested/value1/bar/value2",
		},
	}

	for _, test := range tests {
		u := r.PathTo(test.name, test.args...)
		if u.String() != test.path {
			t.Errorf("Expected path %s but got %s", test.path, u.String())
		}
	}
}
