package ich

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
)

// ensure Router implements the chi.Router interface
var _ chi.Router = (*Mux)(nil)

var namePattern = regexp.MustCompile(`^\([a-zA-Z_-]+\)`)

type namedRoute struct {
	pattern   string
	replacers map[string]*regexp.Regexp
}

type Mux struct {
	chi.Router
	prefix      namedRoute
	namedRoutes map[string]namedRoute
}

func New() *Mux {
	return &Mux{
		Router:      chi.NewRouter(),
		namedRoutes: make(map[string]namedRoute),
	}
}

func (m *Mux) With(middlewares ...func(http.Handler) http.Handler) chi.Router {
	return &Mux{
		Router:      m.Router.With(middlewares...),
		prefix:      m.prefix,
		namedRoutes: m.namedRoutes,
	}
}

func (m *Mux) Group(fn func(r chi.Router)) chi.Router {
	mux := &Mux{
		Router:      m.Router.With(),
		prefix:      m.prefix,
		namedRoutes: m.namedRoutes,
	}
	if fn != nil {
		fn(mux)
	}
	return mux
}

func (m *Mux) Route(pattern string, fn func(r chi.Router)) chi.Router {
	mux := New()
	if fn != nil {
		fn(mux)
	}
	m.Mount(pattern, mux)
	return mux
}

// TODO allow name here (and prefix it to the nested names)?
func (m *Mux) Mount(pattern string, h http.Handler) {
	if mux, ok := h.(*Mux); ok {
		m.Router.Mount(pattern, mux.Router)
		for name, route := range mux.namedRoutes {
			r := namedRoute{
				pattern:   concatPrefix(m.prefix.pattern, concatPrefix(pattern, route.pattern)),
				replacers: make(map[string]*regexp.Regexp),
			}
			for param, replacer := range m.prefix.replacers {
				r.replacers[param] = replacer
			}
			for param, replacer := range compileReplacers(pattern) {
				r.replacers[param] = replacer
			}
			for param, replacer := range route.replacers {
				r.replacers[param] = replacer
			}
			m.namedRoutes[name] = r
		}
	} else {
		m.Router.Mount(pattern, h)
	}
}

func (m *Mux) Handle(pattern string, h http.Handler) {
	m.Router.Handle(m.registerRoute(pattern), h)
}

func (m *Mux) HandleFunc(pattern string, h http.HandlerFunc) {
	m.Router.HandleFunc(m.registerRoute(pattern), h)
}

func (m *Mux) Method(method, pattern string, h http.Handler) {
	m.Router.Method(method, m.registerRoute(pattern), h)
}

func (m *Mux) MethodFunc(method, pattern string, h http.HandlerFunc) {
	m.Router.MethodFunc(method, m.registerRoute(pattern), h)
}

func (m *Mux) Connect(pattern string, h http.HandlerFunc) {
	m.Router.Connect(m.registerRoute(pattern), h)
}

func (m *Mux) Delete(pattern string, h http.HandlerFunc) {
	m.Router.Delete(m.registerRoute(pattern), h)
}

func (m *Mux) Get(pattern string, h http.HandlerFunc) {
	m.Router.Get(m.registerRoute(pattern), h)
}

func (m *Mux) Head(pattern string, h http.HandlerFunc) {
	m.Router.Head(m.registerRoute(pattern), h)
}

func (m *Mux) Options(pattern string, h http.HandlerFunc) {
	m.Router.Options(m.registerRoute(pattern), h)
}

func (m *Mux) Patch(pattern string, h http.HandlerFunc) {
	m.Router.Patch(m.registerRoute(pattern), h)
}

func (m *Mux) Post(pattern string, h http.HandlerFunc) {
	m.Router.Post(m.registerRoute(pattern), h)
}

func (m *Mux) Put(pattern string, h http.HandlerFunc) {
	m.Router.Put(m.registerRoute(pattern), h)
}

func (m *Mux) Trace(pattern string, h http.HandlerFunc) {
	m.Router.Trace(m.registerRoute(pattern), h)
}

// TODO check if param value matches regex
func (m *Mux) URLPath(name string, pairs ...string) (*url.URL, error) {
	route, ok := m.namedRoutes[name]
	if !ok {
		return nil, fmt.Errorf("ich: route '%s' not found", name)
	}

	n := len(pairs)
	if n%2 != 0 {
		return nil, errors.New("ich: number of parameters must be even")
	}

	if n/2 != len(route.replacers) {
		return nil, errors.New("ich: missing parameters")
	}

	p := route.pattern

	for i := 0; i < n; i += 2 {
		replacer := route.replacers[pairs[i]]
		if replacer == nil {
			return nil, fmt.Errorf("ich: unknown param '%s'", pairs[i])
		}
		p = replacer.ReplaceAllString(p, pairs[i+1])
	}

	return &url.URL{
		Path: p,
	}, nil
}

func (m *Mux) URLPathX(name string, pairs ...string) *url.URL {
	u, err := m.URLPath(name, pairs...)
	if err != nil {
		panic(err)
	}
	return u
}

func (m *Mux) registerRoute(namedPattern string) string {
	name, pattern := extractName(namedPattern)
	if name == "" {
		return pattern
	}

	if replacers := compileReplacers(namedPattern); replacers != nil {
		r := namedRoute{
			pattern:   concatPrefix(m.prefix.pattern, pattern),
			replacers: replacers,
		}
		for param, replacer := range m.prefix.replacers {
			r.replacers[param] = replacer
		}
		m.namedRoutes[name] = r
	}

	return pattern
}

func compileReplacers(pattern string) map[string]*regexp.Regexp {
	replacers := make(map[string]*regexp.Regexp)

	// wildcards
	if strings.HasSuffix(pattern, "*") {
		replacers["*"] = regexp.MustCompile(`\*$`)
	}

	// params
	for i := 0; i < len(pattern); i++ {
		char := pattern[i]
		if char == '{' {
			i++
			cc := 1
			ps := i
			pe := i
			for ; i < len(pattern); i++ {
				char = pattern[i]

				if char == '{' {
					cc++
				} else if char == '}' {
					cc--
					if cc == 0 {
						pe = i
						break
					}
				}
			}

			// on unclosed { we just return and let chi do the error handling
			if pe == ps {
				return nil
			}

			param := pattern[ps:pe]

			// param with regex
			if idx := strings.Index(param, ":"); idx >= 0 {
				replacers[param[:idx]] = regexp.MustCompile(`\{` + param[:idx] + `:.{` + fmt.Sprint(len(param[idx+1:])) + `}\}`)
			} else {
				replacers[param] = regexp.MustCompile(`\{` + param + `\}`)
			}
		}
	}

	return replacers
}

func concatPrefix(prefix, pattern string) string {
	if strings.HasSuffix(pattern, "/") {
		prefix += pattern[:len(pattern)-1]
	} else {
		prefix += pattern
	}
	return prefix
}

func extractName(pattern string) (string, string) {
	match := namePattern.FindStringIndex(pattern)
	if match == nil {
		return "", pattern
	}

	name := pattern[1 : match[1]-1]
	origPattern := pattern[match[1]:]
	return name, origPattern
}
