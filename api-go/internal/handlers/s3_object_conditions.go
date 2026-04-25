package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/example/sfree/api-go/internal/manager"
	"github.com/example/sfree/api-go/internal/repository"
	"github.com/gin-gonic/gin"
)

func writeObjectConditionalResponse(c *gin.Context, fileDoc *repository.File) bool {
	status, ok := evaluateObjectReadPreconditions(c.Request, manager.ObjectETag(*fileDoc), fileDoc.CreatedAt)
	if !ok {
		return false
	}
	c.Header("Accept-Ranges", "bytes")
	c.Header("ETag", manager.ObjectETag(*fileDoc))
	c.Header("Last-Modified", fileDoc.CreatedAt.UTC().Format(http.TimeFormat))
	c.Status(status)
	return true
}

func evaluateObjectReadPreconditions(r *http.Request, etag string, modifiedAt time.Time) (int, bool) {
	if raw := strings.TrimSpace(r.Header.Get("If-Match")); raw != "" {
		if !etagListMatches(raw, etag) {
			return http.StatusPreconditionFailed, true
		}
	} else if t, ok := parseConditionalTime(r.Header.Get("If-Unmodified-Since")); ok && objectModifiedAfter(modifiedAt, t) {
		return http.StatusPreconditionFailed, true
	}

	if raw := strings.TrimSpace(r.Header.Get("If-None-Match")); raw != "" {
		if etagListMatches(raw, etag) {
			return http.StatusNotModified, true
		}
		return 0, false
	}
	if t, ok := parseConditionalTime(r.Header.Get("If-Modified-Since")); ok && !objectModifiedAfter(modifiedAt, t) {
		return http.StatusNotModified, true
	}
	return 0, false
}

func parseConditionalTime(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	t, err := http.ParseTime(raw)
	if err != nil {
		return time.Time{}, false
	}
	return t.UTC(), true
}

func objectModifiedAfter(modifiedAt, t time.Time) bool {
	return modifiedAt.UTC().Truncate(time.Second).After(t.UTC())
}

func etagListMatches(raw, etag string) bool {
	for _, candidate := range strings.Split(raw, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if candidate == "*" || candidate == etag {
			return true
		}
	}
	return false
}
