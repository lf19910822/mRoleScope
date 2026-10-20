package httpserver

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ReadinessChecker is intentionally narrow so route tests do not need a live database.
type ReadinessChecker interface {
	Ping(context.Context) error
}

type Router struct {
	readiness ReadinessChecker
}

func New(readiness ReadinessChecker) *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	server := Router{readiness: readiness}
	router.GET("/healthz", server.health)
	router.GET("/readyz", server.ready)
	return router
}

func (r Router) health(context *gin.Context) {
	context.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (r Router) ready(context *gin.Context) {
	if r.readiness == nil || r.readiness.Ping(context.Request.Context()) != nil {
		context.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready"})
		return
	}
	context.JSON(http.StatusOK, gin.H{"status": "ready"})
}
