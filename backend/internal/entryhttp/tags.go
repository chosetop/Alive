package entryhttp

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/p30huiwei/alive/backend/internal/apperr"
	"github.com/p30huiwei/alive/backend/internal/entry"
	"github.com/p30huiwei/alive/backend/internal/httpx"
)

func (h *Handler) ReplaceTags(c *gin.Context) {
	if h.tags == nil {
		httpx.Error(c, &apperr.Error{Code: apperr.CodeInternal, Status: http.StatusInternalServerError, Message: "tag editing is unavailable"})
		return
	}
	authorID, ok := h.resolveAuthor(c)
	if !ok {
		httpx.Error(c, apperr.Unauthorized("authentication required"))
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		httpx.Error(c, apperr.InvalidInput("invalid entry id"))
		return
	}
	var req struct {
		TagIDs   []int64 `json:"tag_ids"`
		Revision int64   `json:"revision"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Revision < 1 {
		httpx.Error(c, apperr.InvalidInput("invalid tag request"))
		return
	}
	revision, err := h.tags.ReplaceTags(c.Request.Context(), id, authorID, req.Revision, req.TagIDs)
	if errors.Is(err, entry.ErrVersionConflict) {
		httpx.Error(c, apperr.Conflict("entry changed since it was loaded"))
		return
	}
	if err != nil {
		httpx.Error(c, apperr.From(err))
		return
	}
	httpx.OK(c, gin.H{"revision": revision, "tag_ids": req.TagIDs})
}
