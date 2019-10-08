package router

import (
	"context"
	"net/http"
	"net/http/httptest"
)

type hr struct {}

func (r hr)Header() http.Header {
	return http.Header{}
}
func (r hr)WriteHeader(i int) {}
func (r hr)Write([]byte) (int, error) {
	return 0, nil
}
func (a *AppRouter) testMiddleware(h http.Handler) {
	a.log.Debugf("testing handler %+v", h)
	req := httptest.NewRequest("get", "/device", nil)
	ctx := context.WithValue(req.Context(), "log", a.log.WithField("test", "handler").Logger)
	res := hr{}
	h.ServeHTTP(res, req.WithContext(ctx))

}