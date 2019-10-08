package pitcher

import (
	"net/http"
	"os"
	"sync"

	"github.com/sirupsen/logrus"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"

	"github.com/dbyington/pitcher/router"
)

var appLog = logrus.New()

func init() {
	appLog.Formatter = &prefixed.TextFormatter{FullTimestamp: true}
	appLog.SetLevel(logrus.DebugLevel)
	appLog.SetOutput(os.Stdout)
}

type App struct {
	*http.Server
	Log   *logrus.Entry
	Route *router.AppRouter
	once    sync.Once
}

// LogLevel sets the log level for the app instance.
func (a *App) LogLevel(level logrus.Level) {
	appLog.SetLevel(level)
}

// RouterLogLevel sets the log level for the router instance.
func (a *App) RouterLogLevel(level logrus.Level) {
	a.Route.SetLevel(level)
}

func (a *App) CONNECT(p string, h http.Handler) {
	a.Route.Add(http.MethodConnect, p, &h)
}

func (a *App) DELETE(p string, h http.Handler) {
	a.Route.Add(http.MethodDelete, p, &h)
}

func (a *App) GET(p string, h http.Handler) {
	a.Route.Add(http.MethodGet, p, &h)
}

func (a *App) HEAD(p string, h http.Handler) {
	a.Route.Add(http.MethodHead, p, &h)
}

func (a *App) OPTIONS(p string, h http.Handler) {
	a.Route.Add(http.MethodOptions, p, &h)
}

func (a *App) PUT(p string, h http.Handler) {
	a.Route.Add(http.MethodPut, p, &h)
}

func (a *App) POST(p string, h http.Handler) {
	a.Route.Add(http.MethodPost, p, &h)
}

func (a *App) PATCH(p string, h http.Handler) {
	a.Route.Add(http.MethodPatch, p, &h)
}

func (a *App) TRACE(p string, h http.Handler) {
	a.Route.Add(http.MethodTrace, p, &h)
}

func (a *App) Use(mw func(http.Handler) http.Handler) {
	a.Route.AddMiddleware(mw)
}

func (a *App) allowMethods() {
	a.once.Do(func() {a.Route.FinishRoutes()})
}

// ListenAndServeTLS configures all routes and starts the TLS https server.
func (a *App) ListenAndServeTLS(cert, key string) error {
	a.allowMethods()
	return a.Server.ListenAndServeTLS(cert, key)
}

// ListenAndServe configures all routes and starts the http server.
func (a *App) ListenAndServe() error {
	a.allowMethods()
	return a.Server.ListenAndServe()
}

// NewApp creates and returns a new app instance with it's own logging.
func NewApp(addr string) *App {
	ar := router.NewRouter()
	ar.ServeMux = http.NewServeMux()
	a := &App{
		Route: ar,
		Log:   appLog.WithField("prefix", "app"),
	}
	a.Server = &http.Server{}
	a.Server.Handler = a.Route
	a.Server.Addr = addr

	return a
}
