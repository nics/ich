package ich

import (
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestPathTo(t *testing.T) {
	r := New()
	r.Get("(foo)/foo/{bar:[a-z-]+}/*", http.NotFound)
	r.Route("/nested/{foo}", func(r chi.Router) {
		if _, ok := r.(*Mux); !ok {
			t.Errorf("Expected *ich.Mux but got %T", r)
		}
		r.Get("(bar)/bar/{baz}", http.NotFound)
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
