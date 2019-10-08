package router

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"github.com/sirupsen/logrus"
)

//const paramExpBraces = `\{(.+)\}`
const paramExpColon = `:(.+)\b`
var paramRegexp = regexp.MustCompile(paramExpColon)

type route struct {
	base                string
	patternMatchMethods map[string][]*matchMethodHandler
	allowMethods        map[string]struct{}
	log                 *logrus.Logger
}
type matchMethodHandler struct {
	matcher *regexp.Regexp
	allowed map[string]*http.Handler
	params  []string
}

func NewRoute(method, pattern string, handler *http.Handler, l *logrus.Logger) (string, *route) {
	r := &route{log: l}
	pattern, params, matcher := r.parsePattern(pattern)
	matchMethod := NewMatchMethodHandler(method, params, matcher, handler)
	patternMatch := make(map[string][]*matchMethodHandler)
	patternMatch[matcher.String()] = []*matchMethodHandler{matchMethod}
	methods := make(map[string]struct{})
	methods[method] = struct{}{}
	return pattern, &route{
		base:                pattern,
		patternMatchMethods: patternMatch,
		allowMethods:        methods,
		log:                 l.WithField("prefix", "route:"+matcher.String()).Logger,
	}
}

func NewMatchMethodHandler(method string, params []string, matcher *regexp.Regexp, handler *http.Handler) *matchMethodHandler {
	methodHandler := make(map[string]*http.Handler)
	methodHandler[method] = handler
	return &matchMethodHandler{
		matcher: matcher,
		allowed: methodHandler,
		params:  params,
	}
}
func (r *route) Add(method, pattern string, handler *http.Handler) string {
	pattern, params, matcher := r.parsePattern(pattern)

	if matchMethodHandlers, ok := r.patternMatchMethods[matcher.String()]; ok {
		r.log.Debugf("add for parsed pattern: %s", pattern)
		for _, matchHandler := range matchMethodHandlers {
			r.log.Debugf("add for method: %s", method)
			if h, ok := matchHandler.allowed[method]; ok {
				if h == handler {
					// Same handler for an existing method with the same pattern.
					r.log.Errorf("handler for pattern %s already exists", pattern)
					return ""
				}
				r.log.Errorf("cannot assign multiple handlers to the same pattern: %s", pattern)
				return ""
			}
		}
		matchMethodHandlers = append(matchMethodHandlers, NewMatchMethodHandler(method, params, matcher, handler))
		r.allowMethods[method] = struct{}{}
		return pattern
	}
	r.patternMatchMethods[matcher.String()] = []*matchMethodHandler{NewMatchMethodHandler(method, params, matcher, handler)}
	r.allowMethods[method] = struct{}{}
	return pattern
}

func (r *route) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.log.Info("Oh boy, gotta figure this one out")
	r.log.Infof("request url path: %s", req.URL.Path)
	if _, ok := r.allowMethods[req.Method]; !ok {
		r.log.Infof("method %s not allowed", req.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	for _, pattern := range r.sortMatchStrings() {
		r.log.Debugf("checking for match against: %s", pattern)

		for _, matcher := range r.patternMatchMethods[pattern] {
			matched := matcher.matcher.FindAllStringSubmatch(req.URL.Path, -1)
			r.log.Debugf("matched %+v", matched)
			if len(matched) > 0 {
				r.log.Debugf("match params: %+v", matcher.params)
				parameters, err := pairParams(matched[0][1:], matcher.params)
				if err != nil {
					r.log.Errorf("error matching param key value pairs: %s", err)
				}
				ctx := context.WithValue(req.Context(), "parameters", parameters)
				r.log.Debugf("run matcher handler: %+v", matcher.allowed[req.Method])
				h := *matcher.allowed[req.Method]
				h.ServeHTTP(w, req.WithContext(ctx))
				return
			}
		}
	}
	http.NotFound(w, req)
}

func pairParams(values, keys []string) (map[string]string, error) {
	if len(values) != len(keys) {
		return nil, errors.New("parameter key value miss match")
	}
	pairs := make(map[string]string)
	for i, k := range keys {
		pairs[k] = values[i]
	}
	return pairs, nil
}

func (r *route) sortMatchStrings() []string {
	var patterns []string
	for pattern := range r.patternMatchMethods {
		patterns = append(patterns, pattern)
	}
	sort.Slice(patterns,
		func(a, b int) bool {
			return patterns[a] > patterns[b]
		})
	r.log.Debugf("sorted patterns: %+v", patterns)
	return patterns
}

func (r *route) parsePattern(pattern string) (string, []string, *regexp.Regexp) {
	var routeParams []string
	routeSlice := []string{""}
	parts := strings.Split(pattern, "/")
	r.log.Debugf("parsing pattern: %s", pattern)
	for _, part := range parts {
		r.log.Debugf("checking route part: %s", part)
		match := paramRegexp.FindStringSubmatch(part)
		r.log.Debugf("matches %d, %+v", len(match), match)
		if len(match) > 0 {

			routeParams = append(routeParams, match[1])
			routeSlice = append(routeSlice, `([[:alnum:]]+)`)
		} else if len(part) > 0 {
			routeSlice = append(routeSlice, part)
		}
	}
	routePath := strings.Join(routeSlice, "/")
	patternRegexp := regexp.MustCompile(routePath)
	s := strings.SplitN(routePath, "(", 2)
	path := s[0]
	if path == "" {
		path = "/"
	}
	r.log.Debugf("returning pattern '%s'", path)
	return path, routeParams, patternRegexp
}
