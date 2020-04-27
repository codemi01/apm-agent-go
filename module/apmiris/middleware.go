//
//  middleware.go
//  go-elastic-apm
//  apmiris module
//
//  Copyright © 2020. All rights reserved.
//

package apmiris

import (
	"net/http"
	"sync"

	"github.com/kataras/iris"

	"go.elastic.co/apm"
	"go.elastic.co/apm/module/apmhttp"
	"go.elastic.co/apm/stacktrace"
)

func init() {
	stacktrace.RegisterLibraryPackage(
		"github.com/kataras/iris",
		"github.com/iris-contrib",
	)
}

func Middleware(engine *iris.Application, o ...Option) iris.Handler {
	m := &middleware{
		engine:         engine,
		tracer:         apm.DefaultTracer,
		requestIgnorer: apmhttp.DefaultServerRequestIgnorer(),
	}
	for _, o := range o {
		o(m)
	}
	return m.handle
}

type middleware struct {
	engine         *iris.Application
	tracer         *apm.Tracer
	requestIgnorer apmhttp.RequestIgnorerFunc

	setRouteMapOnce sync.Once
	routeMap        map[string]map[string]routeInfo
}

type routeInfo struct {
	transactionName string // e.g. "GET /foo"
}

func (m *middleware) handle(c iris.Context) {
	if !m.tracer.Recording() || m.requestIgnorer(c.Request()) {
		c.Next()
		return
	}
	m.setRouteMapOnce.Do(func() {
		routes := m.engine.GetRoutes()
		rm := make(map[string]map[string]routeInfo)
		for _, r := range routes {
			mm := rm[r.Method]
			if mm == nil {
				mm = make(map[string]routeInfo)
				rm[r.Method] = mm
			}
			mm[r.Path] = routeInfo{
				transactionName: r.Method + " " + r.Path,
			}
		}
		m.routeMap = rm
	})

	var requestName string
	handlerName := c.GetCurrentRoute().Path()
	if routeInfo, ok := m.routeMap[c.Request().Method][handlerName]; ok {
		requestName = routeInfo.transactionName
	} else {
		requestName = apmhttp.UnknownRouteRequestName(c.Request())
	}
	tx, req := apmhttp.StartTransaction(m.tracer, requestName, c.Request())
	defer tx.End()

	body := m.tracer.CaptureHTTPRequestBody(req)
	defer func() {
		if v := recover(); v != nil {
			c.StatusCode(http.StatusInternalServerError)
			c.StopExecution()
			e := m.tracer.Recovered(v)
			e.SetTransaction(tx)
			setContext(&e.Context, c, body, req)
			e.Send()
		}
		c.ResponseWriter().Header()
		tx.Result = apmhttp.StatusCodeResult(c.ResponseWriter().StatusCode())

		if tx.Sampled() {
			setContext(&tx.Context, c, body, req)
		}

		body.Discard()
	}()
	c.Next()
}

func setContext(ctx *apm.Context, c iris.Context, body *apm.BodyCapturer, req *http.Request) {
	ctx.SetFramework("iris", iris.Version)
	ctx.SetHTTPRequest(req)
	ctx.SetHTTPRequestBody(body)
	ctx.SetHTTPStatusCode(c.ResponseWriter().StatusCode())
	ctx.SetHTTPResponseHeaders(c.ResponseWriter().Header())
	c.WriteNotModified()
}

type Option func(*middleware)

func WithTracer(t *apm.Tracer) Option {
	if t == nil {
		panic("t == nil")
	}
	return func(m *middleware) {
		m.tracer = t
	}
}

func WithRequestIgnorer(r apmhttp.RequestIgnorerFunc) Option {
	if r == nil {
		r = apmhttp.IgnoreNone
	}
	return func(m *middleware) {
		m.requestIgnorer = r
	}
}
