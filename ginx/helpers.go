package ginx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/githonllc/entigo"
)

// Date is a string type representing a date in "2006-01-02" format.
type Date string

// AsTime parses the Date into a time.Time value.
func (d Date) AsTime() (time.Time, error) {
	return time.Parse("2006-01-02", string(d))
}

// AsTimeOrZero parses the Date into a time.Time value, returning the zero time on error.
func (d Date) AsTimeOrZero() time.Time {
	if t, err := time.Parse("2006-01-02", string(d)); err == nil {
		return t
	}
	return time.Time{}
}

// CopyRequestBodyAsBytes reads the request body into a byte slice and restores
// the body so it can be read again by subsequent handlers.
func CopyRequestBodyAsBytes(c *gin.Context) ([]byte, error) {
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}

	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	return bodyBytes, nil
}

// CopyRequestBody reads the request body and returns it as the appropriate type
// based on the Content-Type header: map[string]any for JSON, string for
// plain text, or raw bytes for other types. The body is restored for subsequent reads.
func CopyRequestBody(c *gin.Context) (any, error) {
	if c.Request == nil || c.Request.Body == nil {
		return nil, nil
	}
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}

	// Restore the original body for subsequent reads
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	// Determine content type
	contentType := c.GetHeader("Content-Type")

	switch {
	case contentType == "application/json":
		var jsonData map[string]any
		if err := json.Unmarshal(bodyBytes, &jsonData); err != nil {
			return nil, err
		}
		return jsonData, nil

	case contentType == "text/plain":
		return string(bodyBytes), nil

	default:
		return bodyBytes, nil
	}
}

// GetUrlParamInt extracts an integer URL parameter, returning defaultValue on error.
func GetUrlParamInt(c *gin.Context, key string, defaultValue int) int {
	str := c.Param(key)
	val, err := strconv.Atoi(str)
	if err != nil {
		return defaultValue
	}
	return val
}

// GetUrlParamID extracts an entigo.ID from a URL parameter.
func GetUrlParamID(c *gin.Context, key string) (entigo.ID, error) {
	str := c.Param(key)
	val, err := strconv.Atoi(str)
	if err != nil {
		return 0, fmt.Errorf("invalid ID %s", str)
	}
	return entigo.ParseID(val)
}

// GetQueryParamInt extracts an integer query parameter, returning defaultValue on error.
func GetQueryParamInt(c *gin.Context, key string, defaultValue int) int {
	str := c.Query(key)
	val, err := strconv.Atoi(str)
	if err != nil {
		return defaultValue
	}
	return val
}

// GetIDFromContext retrieves an entigo.ID from the Gin context by key.
func GetIDFromContext(c *gin.Context, key string) (entigo.ID, error) {
	entIDAny, exists := c.Get(key)
	if !exists {
		return 0, fmt.Errorf("%s doesn't exist in context", key)
	}

	return entigo.ParseID(entIDAny)
}

// RequireContext creates a context.Context from the Gin request context,
// populated with common request metadata (user ID, IP, role flags, etc.).
// Keys use entigo.CtxKey* typed constants to ensure correct retrieval
// in the service layer.
func RequireContext(c *gin.Context) context.Context {
	ctx := c.Request.Context()

	ctx = context.WithValue(ctx, entigo.CtxKeyUserAgent, c.GetHeader("User-Agent"))
	ctx = context.WithValue(ctx, entigo.CtxKeyClientIP, c.ClientIP())
	ctx = context.WithValue(ctx, entigo.CtxKeyRealIP, c.GetHeader("X-Forwarded-For"))
	ctx = context.WithValue(ctx, entigo.CtxKeyIsAdmin, c.GetBool(string(entigo.CtxKeyIsAdmin)))

	if identityID, err := GetIDFromContext(c, string(entigo.CtxKeyIdentityID)); err == nil {
		ctx = context.WithValue(ctx, entigo.CtxKeyIdentityID, identityID)
	}

	if userID, err := GetIDFromContext(c, string(entigo.CtxKeyUserID)); err == nil {
		ctx = context.WithValue(ctx, entigo.CtxKeyUserID, userID)
	} else if apiKeyID, err := GetIDFromContext(c, string(entigo.CtxKeyApiKeyID)); err == nil {
		ctx = context.WithValue(ctx, entigo.CtxKeyApiKeyID, apiKeyID)
	}

	return ctx
}

// IsAdmin reports whether the current request is from an admin user.
func IsAdmin(c *gin.Context) bool {
	return c.GetBool(string(entigo.CtxKeyIsAdmin))
}

// IsCurrentUser reports whether the given userId matches the authenticated user ID,
// or whether the caller is an admin.
func IsCurrentUser(c *gin.Context, userId entigo.ID) bool {
	if id, err := GetIDFromContext(c, string(entigo.CtxKeyUserID)); err == nil {
		return id == userId
	}
	return c.GetBool(string(entigo.CtxKeyIsAdmin))
}

// IsAdminOrCurrentUser reports whether the caller is an admin or the specified user.
func IsAdminOrCurrentUser(c *gin.Context, userId entigo.ID) bool {
	return IsAdmin(c) || IsCurrentUser(c, userId)
}

// QueryParamsToQueryMap converts Gin request query parameters to an entigo.QueryMap.
func QueryParamsToQueryMap(c *gin.Context) entigo.QueryMap {
	queryMap := make(entigo.QueryMap)
	for key, values := range c.Request.URL.Query() {
		if len(values) > 0 {
			queryMap[key] = values
		}
	}
	return queryMap
}
