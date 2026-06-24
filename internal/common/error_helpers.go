package common

import (
	"database/sql"
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

// HandleDatabaseError converts database errors to appropriate OpenStack errors
func HandleDatabaseError(c *gin.Context, err error, resource string) {
	if errors.Is(err, sql.ErrNoRows) {
		SendError(c, NewNotFoundError(resource))
		return
	}

	// Log the database error
	log.Error().
		Err(err).
		Str("resource", resource).
		Msg("Database error occurred")

	SendError(c, NewDatabaseError("query", resource, err))
}

// AbortWithError sends the error response and aborts the request chain
func AbortWithError(c *gin.Context, err *OpenStackError) {
	SendError(c, err)
	c.Abort()
}
