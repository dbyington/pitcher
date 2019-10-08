package router

import (
	"context"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
)

var routeLog = logrus.New()

func init() {
	routeLog.Formatter = &prefixed.TextFormatter{FullTimestamp: true}
	routeLog.SetLevel(logrus.DebugLevel)
	routeLog.SetOutput(os.Stdout)
}

type AppRouter struct {
	*http.ServeMux
	log        *logrus.Logger
	routes     map[string]*route
	routePatterns []string
	methods    map[string]struct{}
	middleware []func(http.Handler) http.Handler
}

type Router interface {
	Add(string, string, http.Handler)
	AddMiddleware(func(http.Handler) http.Handler)
}
func NewRouter() *AppRouter {
	return &AppRouter{
		log:     routeLog,
		routes:  make(map[string]*route),
		methods: make(map[string]struct{}),
	}
}

func (a *AppRouter) SetLevel(l logrus.Level) {
	a.log.SetLevel(l)
}

func (a *AppRouter) Add(method, pattern string, handler *http.Handler) {
	a.methods[method] = struct{}{}
	p, route := NewRoute(method, pattern, handler, routeLog)
	a.routePatterns = append(a.routePatterns, p)
	if _, ok := a.routes[p]; ok {
		a.routes[p].Add(method, pattern, handler)
	} else {
		a.routes[p] = route
	}
}

func (a *AppRouter) methodHandler(pattern string, handler http.Handler) http.Handler {
	if route, ok := a.routes[pattern]; ok {
		return http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
			log := r.Context().Value("log").(*logrus.Logger)
			log.Debugf("(%s) handle %s for %s", r.Context().Value("requestID"), r.Method, r.RequestURI)
			if _, ok := route.allowMethods[r.Method]; ok {

				log.Debugf("(%s) routing for %s(%s), method %s allowed", r.Context().Value("requestID"), r.RequestURI, pattern, r.Method)
				log.Debug("adding logger to request context")

				ctx := context.WithValue(r.Context(), "log", log)

				handler.ServeHTTP(w, r.WithContext(ctx))
				log.Debug("request served")
				return
			}
			w.WriteHeader(http.StatusMethodNotAllowed)
		})
	}
	return nil
}

// ServeHTTP fulfills the http.Handler interface requirement.
func (a *AppRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var ctx context.Context

	h, s := a.Handler(r)

	requestID := uuid.New().String()
	requestTimestamp := time.Now()
	ctx = context.WithValue(r.Context(), "requestID", requestID)
	ctx = context.WithValue(ctx, "requestTimestamp", requestTimestamp)

	logFields := make(map[string]interface{})
	logFields["prefix"] = "route|" + r.URL.Path
	logFields["route"] = r.RequestURI
	logFields["requestID"] = requestID
	log := a.log.WithFields(logFields)
	ctx = context.WithValue(ctx, "log", log.Logger)

	log.Debugf("request started, figuring out pattern from: %s. try %s", r.URL.Path, s)
	h.ServeHTTP(w, r.WithContext(ctx))
	log.WithField("finishedNanoseconds", time.Now().Sub(requestTimestamp).Nanoseconds()).Debugf("request is done")
}

func (a *AppRouter) AddMiddleware(handler func(http.Handler) http.Handler) {
	a.log.Debugf("adding middleware: %+v", handler)
	a.middleware = append(a.middleware, handler)
	a.log.Debugf("middleware now: %+v", a.middleware)
}

func (a *AppRouter) FinishRoutes() {
	a.sortRoutes()
	a.log.Debugf("ROUTES: %+v", a.routes)
	for pattern, route := range a.routes {
		a.log.Debugf("registering handler for route (%+v)%s with %+v", route.base, pattern, route.allowMethods)
		var final http.Handler
		final = route
		a.log.Debugf("final middleware: %+v", final)
		for i := len(a.middleware)-1; i >= 0; i-- {
			t := final
			final = a.middleware[i](final)
			a.log.Debugf("added middleware %+v to handler: %+v", t, a.middleware[i])
		}
		final = a.methodHandler(pattern, final)
		a.log.Debugf("created handler %+v for %s", final.ServeHTTP, pattern)
		a.HandleFunc(pattern, final.ServeHTTP)
	}
}

func (a *AppRouter) sortRoutes() {
	patterns := a.routePatterns
	sort.Slice(patterns, func(a,b int) bool {return patterns[a] > patterns[b]})
	a.routePatterns = patterns
}

