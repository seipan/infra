package handler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/yamasaki/static-server/internal/cache"
	"github.com/yamasaki/static-server/internal/storage"
)

type StaticHandler struct {
	storage *storage.MinioStorage
	cache   *cache.Cache
	logger  *logrus.Logger

	// maxCacheableBytes is the largest object body kept in memory; anything
	// bigger is streamed straight through to the client.
	maxCacheableBytes int64
}

func NewStaticHandler(storage *storage.MinioStorage, cache *cache.Cache, maxCacheableBytes int64, logger *logrus.Logger) *StaticHandler {
	return &StaticHandler{
		storage:           storage,
		cache:             cache,
		logger:            logger,
		maxCacheableBytes: maxCacheableBytes,
	}
}

func (h *StaticHandler) ServeFiles(c *gin.Context) {
	// This runs as gin's NoRoute handler, which catches every method, so the
	// two we actually serve have to be checked here.
	if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "Method not allowed"})
		return
	}

	host := c.Request.Host
	bucket := extractBucketFromHost(host)
	if bucket == "" {
		h.logger.WithField("host", host).Error("Could not extract bucket from host")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid host"})
		return
	}
	objectPath := strings.TrimPrefix(c.Request.URL.Path, "/")
	if strings.HasSuffix(objectPath, "/") || objectPath == "" {
		objectPath = path.Join(objectPath, "index.html")
	}
	// Add bucket name as prefix since objects are stored with bucket prefix
	objectPath = path.Join(bucket, objectPath)

	h.logger.WithFields(logrus.Fields{
		"bucket":     bucket,
		"objectPath": objectPath,
		"host":       host,
		"url":        c.Request.URL.Path,
	}).Debug("Attempting to serve file")

	if entry, ok := h.cache.Get(objectPath); ok {
		h.serveEntry(c, entry)
		return
	}

	// The request context rather than the gin one, so that a client hanging up
	// cancels the fetch instead of leaving it to run to completion.
	obj, resolvedPath, err := h.fetch(c.Request.Context(), bucket, objectPath)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			h.logger.WithFields(logrus.Fields{
				"bucket": bucket,
				"object": objectPath,
			}).Warn("Object not found")
			entry := cache.Entry{Found: false}
			h.cache.Put(objectPath, entry)
			h.serveEntry(c, entry)
			return
		}
		h.logger.WithError(err).WithFields(logrus.Fields{
			"bucket": bucket,
			"object": objectPath,
		}).Error("Failed to get object from storage")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}
	defer obj.Body.Close()

	contentType := getContentType(resolvedPath)

	// Objects small enough to cache are buffered so the next request for them
	// costs nothing; larger ones are streamed and left out of the cache.
	if obj.Info.Size >= 0 && obj.Info.Size <= h.maxCacheableBytes {
		body, err := io.ReadAll(obj.Body)
		if err != nil {
			h.logger.WithError(err).WithFields(logrus.Fields{
				"bucket": bucket,
				"object": resolvedPath,
			}).Error("Failed to read object from storage")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}
		entry := cache.Entry{Body: body, ContentType: contentType, Found: true}
		h.cache.Put(objectPath, entry)
		h.serveEntry(c, entry)
		return
	}

	h.setHeaders(c, contentType)
	if _, err := io.Copy(c.Writer, obj.Body); err != nil {
		h.logger.WithError(err).Error("Failed to write response")
	}
}

// fetch resolves objectPath, falling back to the same path with .html appended
// when the original is missing and does not already end in .html. It returns the
// open object along with the path it was actually found at.
func (h *StaticHandler) fetch(ctx context.Context, bucket, objectPath string) (*storage.Object, string, error) {
	obj, err := h.storage.GetObject(ctx, bucket, objectPath)
	if err == nil {
		return obj, objectPath, nil
	}
	if !errors.Is(err, storage.ErrNotFound) || strings.HasSuffix(objectPath, ".html") {
		return nil, "", err
	}

	htmlPath := objectPath + ".html"
	obj, err = h.storage.GetObject(ctx, bucket, htmlPath)
	if err != nil {
		return nil, "", err
	}
	return obj, htmlPath, nil
}

// serveEntry writes a cached or freshly buffered lookup result.
func (h *StaticHandler) serveEntry(c *gin.Context, entry cache.Entry) {
	if !entry.Found {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
		return
	}
	h.setHeaders(c, entry.ContentType)
	if _, err := c.Writer.Write(entry.Body); err != nil {
		h.logger.WithError(err).Error("Failed to write response")
	}
}

func (h *StaticHandler) setHeaders(c *gin.Context, contentType string) {
	c.Header("Content-Type", contentType)
	c.Header("Cache-Control", "public, max-age=3600")
}

func extractBucketFromHost(host string) string {
	parts := strings.Split(host, ".")
	if len(parts) > 2 {
		return parts[0]
	}
	return ""
}

func getContentType(filename string) string {
	ext := strings.ToLower(path.Ext(filename))
	switch ext {
	case ".html":
		return "text/html; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".js":
		return "application/javascript; charset=utf-8"
	case ".json":
		return "application/json; charset=utf-8"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".ico":
		return "image/x-icon"
	case ".webp":
		return "image/webp"
	case ".woff":
		return "font/woff"
	case ".woff2":
		return "font/woff2"
	case ".ttf":
		return "font/ttf"
	case ".eot":
		return "application/vnd.ms-fontobject"
	default:
		return "application/octet-stream"
	}
}
