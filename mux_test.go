package ich

import (
	"net/http"
	"net/url"
	"testing"
)

func TestBuildPath(t *testing.T) {
	r := New()
	r.Get("/", http.NotFound).Name("home")
	r.Get("/foo/{bar:[a-z-]+}/*", http.NotFound).Name("foo")
	r.Route("/nested/{foo}", func(r *Mux) {
		r.Get("/bar/{baz}", http.NotFound).Name("bar")
	})

	tests := []struct {
		name string
		args []any
		path string
	}{
		{
			"home",
			[]any{},
			"/",
		},
		{
			"home",
			[]any{"foo", "bar"},
			"/?foo=bar",
		},
		{
			"foo",
			[]any{"bar", "value", "*", "wild/card"},
			"/foo/value/wild/card",
		},
		{
			"bar",
			[]any{"baz", "value2", []string{"foo", "value1"}},
			"/nested/value1/bar/value2",
		},
		{
			"bar",
			[]any{url.Values{"q": []string{"qvalue1", "qvalue2"}}, "baz", "value2", []string{"foo", "value1"}},
			"/nested/value1/bar/value2?q=qvalue1&q=qvalue2",
		},
	}

	for _, test := range tests {
		u, err := r.BuildPath(test.name, test.args...)
		if err != nil {
			t.Error(err)
		} else if u.String() != test.path {
			t.Errorf("Expected path '%s' but got '%s'", test.path, u.String())
		}
	}
}
