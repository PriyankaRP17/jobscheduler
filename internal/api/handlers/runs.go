package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yourname/jobscheduler/internal/api/middleware"
	"github.com/yourname/jobscheduler/internal/domain"
	"github.com/yourname/jobscheduler/internal/repository"
	"github.com/yourname/jobscheduler/internal/service"
)

// RunHandler handles run-related HTTP requests.
type RunHandler struct {
	svc *service.RunService
}

// NewRunHandler creates a RunHandler.
func NewRunHandler(svc *service.RunService) *RunHandler {
	return &RunHandler{svc: svc}
}

// Trigger handles POST /api/v1/jobs/:id/trigger
func (h *RunHandler) Trigger(c *gin.Context) {
	jobID := c.Param("id")
	ownerID := middleware.GetOwnerID(c)

	run, err := h.svc.TriggerJob(c.Request.Context(), jobID, ownerID)
	if err != nil {
		if err.Error() == "job not found" {
			respondError(c, http.StatusNotFound, "NOT_FOUND", "job not found")
			return
		}
		if err.Error() == "job is paused" {
			respondError(c, http.StatusConflict, "JOB_PAUSED", "cannot trigger a paused job")
			return
		}
		respondError(c, http.StatusInternalServerError, "TRIGGER_FAILED", err.Error())
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"run_id": run.ID, "state": run.State})
}

// Get handles GET /api/v1/runs/:id
func (h *RunHandler) Get(c *gin.Context) {
	runID := c.Param("id")
	ownerID := middleware.GetOwnerID(c)

	run, err := h.svc.GetRun(c.Request.Context(), runID, ownerID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	if run == nil {
		respondError(c, http.StatusNotFound, "NOT_FOUND", "run not found")
		return
	}
	c.JSON(http.StatusOK, run)
}

// ListByJob handles GET /api/v1/jobs/:id/runs
func (h *RunHandler) ListByJob(c *gin.Context) {
	jobID := c.Param("id")
	ownerID := middleware.GetOwnerID(c)

	runs, nextCursor, err := h.svc.ListRuns(c.Request.Context(), repository.ListRunsFilter{
		JobID:   jobID,
		OwnerID: ownerID,
		Limit:   20,
		Cursor:  c.Query("cursor"),
	})
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": runs, "next_cursor": nextCursor})
}

// ListAll handles GET /api/v1/runs (supports ?state=dead for dead-letter view)
func (h *RunHandler) ListAll(c *gin.Context) {
	ownerID := middleware.GetOwnerID(c)
	f := repository.ListRunsFilter{
		OwnerID: ownerID,
		State:   domain.RunState(c.Query("state")),
		Limit:   20,
		Cursor:  c.Query("cursor"),
	}
	runs, nextCursor, err := h.svc.ListRuns(c.Request.Context(), f)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": runs, "next_cursor": nextCursor})
}

// Retry handles POST /api/v1/runs/:id/retry
func (h *RunHandler) Retry(c *gin.Context) {
	runID := c.Param("id")
	ownerID := middleware.GetOwnerID(c)

	run, err := h.svc.RetryDeadRun(c.Request.Context(), runID, ownerID)
	if err != nil {
		respondError(c, http.StatusBadRequest, "RETRY_FAILED", err.Error())
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"run_id": run.ID, "state": run.State})
}
