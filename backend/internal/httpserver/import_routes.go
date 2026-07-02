package httpserver

import (
	"io"
	"net/http"

	"github.com/Laisky/errors/v2"
	gmw "github.com/Laisky/gin-middlewares/v7"
	glog "github.com/Laisky/go-utils/v6/log"
	"github.com/Laisky/zap"
	"github.com/gin-gonic/gin"

	"github.com/Laisky/Accounting/backend/internal/audit"
	"github.com/Laisky/Accounting/backend/internal/auth"
	importsvc "github.com/Laisky/Accounting/backend/internal/imports"
)

const maxImportUploadBytes = 6 * 1024 * 1024

// registerImportRoutes receives an API group and registers protected import preview endpoints.
func registerImportRoutes(api *gin.RouterGroup, importService *importsvc.Service, auditService *audit.Service) {
	api.POST("/imports/wacai/preview", RequireSession(), func(c *gin.Context) {
		log := gmw.GetLogger(c)

		actor, ok := auth.ActorFromContext(c.Request.Context())
		if !ok {
			log.Debug("actor context missing")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}

		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxImportUploadBytes)
		fileHeader, err := c.FormFile("file")
		if err != nil {
			log.Debug("import file missing", zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "import file is required"})
			return
		}
		file, err := fileHeader.Open()
		if err != nil {
			log.Debug("open import file failed", zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "import file is invalid"})
			return
		}
		defer func() {
			if closeErr := file.Close(); closeErr != nil {
				log.Debug("close import file failed", zap.Error(closeErr))
			}
		}()

		data, err := io.ReadAll(file)
		if err != nil {
			log.Debug("read import file failed", zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "import file is invalid"})
			return
		}

		batch, err := importService.PreviewWacaiCSV(c.Request.Context(), importsvc.PreviewRequest{
			Actor:       importsvc.Actor{UserID: actor.UserID},
			Filename:    fileHeader.Filename,
			ContentType: fileHeader.Header.Get("Content-Type"),
			Data:        data,
		})
		if err != nil {
			respondImportError(c, log, err)
			return
		}

		log.Debug("wacai import preview created", zap.String("batch_id", batch.ID), zap.String("user_id", actor.UserID))
		recordAuditEvent(c, auditService, audit.RecordRequest{
			ActorID:    actor.UserID,
			ActorEmail: actor.Email,
			Action:     audit.ActionImportPreviewCreated,
			TargetType: "import_batch",
			TargetID:   batch.ID,
			Metadata: map[string]string{
				"source":   batch.Source,
				"filename": batch.Filename,
			},
		})
		c.JSON(http.StatusCreated, batch)
	})
}

// respondImportError receives an import service error and writes a stable API response.
func respondImportError(c *gin.Context, log glog.Logger, err error) {
	switch {
	case errors.Is(err, importsvc.ErrInvalidInput):
		log.Debug("import input rejected", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid import input"})
	case errors.Is(err, importsvc.ErrNotFound):
		log.Debug("import resource not found", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "import resource not found"})
	default:
		log.Debug("import request failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "import request failed"})
	}
}
