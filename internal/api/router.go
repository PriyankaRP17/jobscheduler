package api

import (
	"github.com/gin-gonic/gin"
	"github.com/yourname/jobscheduler/internal/api/handlers"
	"github.com/yourname/jobscheduler/internal/api/middleware"
)

// NewRouter sets up all routes and middleware.
func NewRouter(
	jobHandler *handlers.JobHandler,
	runHandler *handlers.RunHandler,
	workerHandler *handlers.WorkerHandler,
) *gin.Engine {
	r := gin.New()

	// Global middleware
	r.Use(middleware.Logger())
	r.Use(middleware.Recovery())

	// Health (no auth)
	r.GET("/health", handlers.Health)
	r.GET("/ready", handlers.Ready)

	// API v1
	v1 := r.Group("/api/v1")
	{
		// Jobs
		jobs := v1.Group("/jobs")
		{
			jobs.POST("", jobHandler.Create)
			jobs.GET("", jobHandler.List)
			jobs.GET("/:id", jobHandler.Get)
			jobs.PATCH("/:id", jobHandler.Update)
			jobs.DELETE("/:id", jobHandler.Delete)
			jobs.POST("/:id/trigger", runHandler.Trigger)
			jobs.GET("/:id/runs", runHandler.ListByJob)
		}

		// Runs
		runs := v1.Group("/runs")
		{
			runs.GET("/:id", runHandler.Get)
			runs.POST("/:id/retry", runHandler.Retry)
			runs.GET("", runHandler.ListAll)
		}

		// Workers
		v1.GET("/workers", workerHandler.List)
	}

	return r
}
