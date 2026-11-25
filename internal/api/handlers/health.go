package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yourname/jobscheduler/internal/repository"
)

// WorkerHandler handles worker-related requests.
type WorkerHandler struct {
	workerRepo repository.WorkerRepository
}

// NewWorkerHandler creates a WorkerHandler.
func NewWorkerHandler(workerRepo repository.WorkerRepository) *WorkerHandler {
	return &WorkerHandler{workerRepo: workerRepo}
}

// List handles GET /api/v1/workers
func (h *WorkerHandler) List(c *gin.Context) {
	workers, err := h.workerRepo.ListAll(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": workers})
}

// Health handles GET /health — liveness probe.
func Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Ready handles GET /ready — readiness probe.
// Week 5 will add real Mongo and NATS ping checks here.
func Ready(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}
