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

type namedRoute struct {
	method    string
	pattern   string
	replacers map[string]*regexp.Regexp
}

type Mux struct {
	chi.Router
	prefix      namedRoute
	namedRoutes map[string]namedRoute
	routeNames  map[string]string
}

func New() *Mux {
	return &Mux{
		Router:      chi.NewRouter(),
		namedRoutes: make(map[string]namedRoute),
		routeNames:  make(map[string]string),
	}
}

func (m *Mux) With(middlewares ...func(http.Handler) http.Handler) *Mux {
	return &Mux{
		Router:      m.Router.With(middlewares...),
		prefix:      m.prefix,
		namedRoutes: m.namedRoutes,
		routeNames:  m.routeNames,
	}
}

func (m *Mux) Group(fn func(r *Mux)) *Mux {
	mux := &Mux{
		Router:      m.Router.With(),
		prefix:      m.prefix,
		namedRoutes: m.namedRoutes,
		routeNames:  m.routeNames,
	}
	if fn != nil {
		fn(mux)
	}
	return mux
}

func (m *Mux) Route(pattern string, fn func(r *Mux)) *Mux {
	mux := New()
	if fn != nil {
		fn(mux)
	}
	m.Mount(pattern, mux)
	return mux
}

func (m *Mux) Mount(pattern string, h http.Handler) {
	if mux, ok := h.(*Mux); ok {
		m.Router.Mount(pattern, mux.Router)
		for name, r := range mux.namedRoutes {
			rr := namedRoute{
				pattern:   concatPrefix(m.prefix.pattern, concatPrefix(pattern, r.pattern)),
				replacers: make(map[string]*regexp.Regexp),
			}
			for param, replacer := range m.prefix.replacers {
				rr.replacers[param] = replacer
			}
			for param, replacer := range compileReplacers(pattern) {
				rr.replacers[param] = replacer
			}
			for param, replacer := range r.replacers {
				rr.replacers[param] = replacer
			}
			m.namedRoutes[name] = rr
			m.routeNames[rr.method+rr.pattern] = name
		}
	} else {
		m.Router.Mount(pattern, h)
	}
}

type Builder struct {
	prefix      namedRoute
	namedRoutes map[string]namedRoute
	routeNames  map[string]string
	method      string
	pattern     string
}

func (b Builder) Name(name string) {
	if name == "" {
		return
	}

	if replacers := compileReplacers(b.pattern); replacers != nil {
		r := namedRoute{
			method:    b.method,
			pattern:   concatPrefix(b.prefix.pattern, b.pattern),
			replacers: replacers,
		}
		for param, replacer := range b.prefix.replacers {
			r.replacers[param] = replacer
		}
		b.namedRoutes[name] = r
		b.routeNames[b.method+b.pattern] = name
	}
}

func (m *Mux) builder(method, pattern string) Builder {
	return Builder{
		namedRoutes: m.namedRoutes,
		routeNames:  m.routeNames,
		prefix:      m.prefix,
		method:      method,
		pattern:     pattern,
	}
}

func (m *Mux) Handle(pattern string, h http.Handler) Builder {
	m.Router.Handle(pattern, h)
	return m.builder("", pattern)
}

func (m *Mux) HandleFunc(pattern string, h http.HandlerFunc) Builder {
	m.Router.HandleFunc(pattern, h)
	return m.builder("", pattern)
}

func (m *Mux) Method(method, pattern string, h http.Handler) Builder {
	m.Router.Method(method, pattern, h)
	return m.builder(method, pattern)
}

func (m *Mux) MethodFunc(method, pattern string, h http.HandlerFunc) Builder {
	m.Router.MethodFunc(method, pattern, h)
	return m.builder(method, pattern)
}

func (m *Mux) Connect(pattern string, h http.HandlerFunc) Builder {
	m.Router.Connect(pattern, h)
	return m.builder("CONNECT", pattern)
}

func (m *Mux) Delete(pattern string, h http.HandlerFunc) Builder {
	m.Router.Delete(pattern, h)
	return m.builder("DELETE", pattern)
}

func (m *Mux) Get(pattern string, h http.HandlerFunc) Builder {
	m.Router.Get(pattern, h)
	return m.builder("GET", pattern)
}

func (m *Mux) Head(pattern string, h http.HandlerFunc) Builder {
	m.Router.Head(pattern, h)
	return m.builder("HEAD", pattern)
}

func (m *Mux) Options(pattern string, h http.HandlerFunc) Builder {
	m.Router.Options(pattern, h)
	return m.builder("OPTIONS", pattern)
}

func (m *Mux) Patch(pattern string, h http.HandlerFunc) Builder {
	m.Router.Patch(pattern, h)
	return m.builder("PATCH", pattern)
}

func (m *Mux) Post(pattern string, h http.HandlerFunc) Builder {
	m.Router.Post(pattern, h)
	return m.builder("POST", pattern)
}

func (m *Mux) Put(pattern string, h http.HandlerFunc) Builder {
	m.Router.Put(pattern, h)
	return m.builder("PUT", pattern)
}

func (m *Mux) Trace(pattern string, h http.HandlerFunc) Builder {
	m.Router.Trace(pattern, h)
	return m.builder("TRACE", pattern)
}

func (m *Mux) RouteName(method, pattern string) string {
	return m.routeNames[method+pattern]
}

// TODO check if param value matches regex
func (m *Mux) BuildPath(name string, params ...any) (*url.URL, error) {
	route, ok := m.namedRoutes[name]
	if !ok {
		return nil, fmt.Errorf("ich: route '%s' not found", name)
	}

	pattern := route.pattern
	remaining := len(route.replacers)
	var q url.Values

	for i := 0; i < len(params); i += 1 {
		switch param := params[i].(type) {
		case string:
			if len(params) == i+1 {
				return nil, errors.New("ich: string path params must be followed by another string")
			}
			i++
			nextParam, ok := params[i].(string)
			if !ok {
				return nil, errors.New("ich: string path params must be followed by another string")
			}
			if replacer := route.replacers[param]; replacer != nil {
				pattern = replacer.ReplaceAllString(pattern, nextParam)
				remaining--
			} else {
				if q == nil {
					q = make(url.Values)
				}
				q.Add(param, nextParam)
			}
		case []string:
			if len(param)%2 != 0 {
				return nil, errors.New("ich: string path params must be followed by another string")
			}
			for j := 0; j < len(param); j += 2 {
				if replacer := route.replacers[param[j]]; replacer != nil {
					pattern = replacer.ReplaceAllString(pattern, param[j+1])
					remaining--
				} else {
					if q == nil {
						q = make(url.Values)
					}
					q.Add(param[j], param[j+1])
				}
			}
		case url.Values:
			for k, vals := range param {
				if q == nil {
					q = make(url.Values)
				}
				for _, val := range vals {
					q.Add(k, val)
				}
			}
		default:
			return nil, errors.New("ich: path params must be string, []string or url.Values")
		}
	}

	if remaining > 0 {
		return nil, errors.New("ich: missing route params")
	}

	u := &url.URL{
		Path: pattern,
	}

	if q != nil {
		u.RawQuery = q.Encode()
	}

	return u, nil
}

func (m *Mux) Path(name string, pairs ...any) *url.URL {
	u, err := m.BuildPath(name, pairs...)
	if err != nil {
		panic(err)
	}
	return u
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
	if len(pattern) > 1 && strings.HasSuffix(pattern, "/") {
		prefix += pattern[:len(pattern)-1]
	} else {
		prefix += pattern
	}
	return prefix
}
