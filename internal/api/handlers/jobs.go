package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yourname/jobscheduler/internal/api/middleware"
	"github.com/yourname/jobscheduler/internal/domain"
	"github.com/yourname/jobscheduler/internal/repository"
	"github.com/yourname/jobscheduler/internal/service"
)

// JobHandler handles job-related HTTP requests.
type JobHandler struct {
	svc *service.JobService
}

// NewJobHandler creates a JobHandler.
func NewJobHandler(svc *service.JobService) *JobHandler {
	return &JobHandler{svc: svc}
}

// createJobRequest is the JSON body for POST /jobs.
type createJobRequest struct {
	Name     string             `json:"name"`
	Type     string             `json:"type"`
	Payload  map[string]any     `json:"payload"`
	Schedule domain.Schedule    `json:"schedule"`
	Policy   domain.RetryPolicy `json:"policy"`
}

// Create handles POST /api/v1/jobs
func (h *JobHandler) Create(c *gin.Context) {
	var req createJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}

	ownerID := middleware.GetOwnerID(c)
	job, err := h.svc.CreateJob(c.Request.Context(), service.CreateJobInput{
		OwnerID:  ownerID,
		Name:     req.Name,
		Type:     req.Type,
		Payload:  req.Payload,
		Schedule: req.Schedule,
		Policy:   req.Policy,
	})
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_JOB", err.Error())
		return
	}

	c.JSON(http.StatusCreated, job)
}

// Get handles GET /api/v1/jobs/:id
func (h *JobHandler) Get(c *gin.Context) {
	id := c.Param("id")
	ownerID := middleware.GetOwnerID(c)

	job, err := h.svc.GetJob(c.Request.Context(), id, ownerID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	if job == nil {
		respondError(c, http.StatusNotFound, "NOT_FOUND", "job not found")
		return
	}
	c.JSON(http.StatusOK, job)
}

// List handles GET /api/v1/jobs
func (h *JobHandler) List(c *gin.Context) {
	ownerID := middleware.GetOwnerID(c)
	f := repository.ListJobsFilter{
		OwnerID: ownerID,
		Status:  domain.JobStatus(c.Query("status")),
		Limit:   20,
		Cursor:  c.Query("cursor"),
	}

	jobs, nextCursor, err := h.svc.ListJobs(c.Request.Context(), f)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items":       jobs,
		"next_cursor": nextCursor,
	})
}

// updateJobRequest is the JSON body for PATCH /jobs/:id.
type updateJobRequest struct {
	Status *string `json:"status"` // "paused" | "active"
}

// Update handles PATCH /api/v1/jobs/:id (pause/resume for now)
func (h *JobHandler) Update(c *gin.Context) {
	id := c.Param("id")
	ownerID := middleware.GetOwnerID(c)

	var req updateJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}

	ctx := c.Request.Context()
	var err error
	if req.Status != nil {
		switch *req.Status {
		case "paused":
			err = h.svc.PauseJob(ctx, id, ownerID)
		case "active":
			err = h.svc.ResumeJob(ctx, id, ownerID)
		default:
			respondError(c, http.StatusBadRequest, "INVALID_STATUS", "status must be active or paused")
			return
		}
	}
	if err != nil {
		respondError(c, http.StatusBadRequest, "UPDATE_FAILED", err.Error())
		return
	}

	job, _ := h.svc.GetJob(ctx, id, ownerID)
	c.JSON(http.StatusOK, job)
}

// Delete handles DELETE /api/v1/jobs/:id
func (h *JobHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	ownerID := middleware.GetOwnerID(c)

	if err := h.svc.DeleteJob(c.Request.Context(), id, ownerID); err != nil {
		respondError(c, http.StatusInternalServerError, "DELETE_FAILED", err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

// respondError writes a consistent error envelope.
func respondError(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{
		"error": gin.H{
			"code":    code,
			"message": message,
		},
	})
}
