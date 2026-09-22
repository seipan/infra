package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/yamasaki/static-server/internal/cache"
	"github.com/yamasaki/static-server/internal/handler"
	"github.com/yamasaki/static-server/internal/storage"
)

// healthPath answers the liveness probe. It deliberately touches nothing but
// the process itself: a probe that reaches MinIO turns a slow bucket into a
// restart loop, and takes every replica down at once.
const healthPath = "/healthz"

func main() {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetLevel(logrus.DebugLevel)

	endpoint := getEnv("MINIO_ENDPOINT", "minio.minio.svc.cluster.local:9000")
	accessKey := getEnv("MINIO_ACCESS_KEY", "")
	secretKey := getEnv("MINIO_SECRET_KEY", "")
	useSSL := getEnvBool("MINIO_USE_SSL", false)
	port := getEnv("PORT", "8080")

	cacheTTL := getEnvDuration("CACHE_TTL", time.Minute)
	cacheNegativeTTL := getEnvDuration("CACHE_NEGATIVE_TTL", 10*time.Second)
	cacheMaxBytes := getEnvInt64("CACHE_MAX_BYTES", 16<<20)
	cacheMaxObjectBytes := getEnvInt64("CACHE_MAX_OBJECT_BYTES", 1<<20)

	if accessKey == "" || secretKey == "" {
		logger.Fatal("MINIO_ACCESS_KEY and MINIO_SECRET_KEY must be set")
	}

	minioStorage, err := storage.NewMinioStorage(endpoint, accessKey, secretKey, useSSL, logger)
	if err != nil {
		logger.WithError(err).Fatal("Failed to initialize MinIO storage")
	}

	objectCache := cache.New(cacheTTL, cacheNegativeTTL, cacheMaxBytes)
	staticHandler := handler.NewStaticHandler(minioStorage, objectCache, cacheMaxObjectBytes, logger)

	gin.SetMode(gin.ReleaseMode)
	r := newRouter(staticHandler, logger)

	logger.WithFields(logrus.Fields{
		"port":                port,
		"cacheTTL":            cacheTTL.String(),
		"cacheNegativeTTL":    cacheNegativeTTL.String(),
		"cacheMaxBytes":       cacheMaxBytes,
		"cacheMaxObjectBytes": cacheMaxObjectBytes,
	}).Info("Starting server")
	if err := r.Run(fmt.Sprintf(":%s", port)); err != nil {
		logger.WithError(err).Fatal("Server failed")
	}
}

// newRouter wires the health endpoint and the static file handler together.
func newRouter(staticHandler *handler.StaticHandler, logger *logrus.Logger) *gin.Engine {
	r := gin.New()

	r.Use(loggingMiddleware(logger))
	r.Use(gin.Recovery())

	r.GET(healthPath, healthz)
	r.HEAD(healthPath, healthz)

	// Static files are served from NoRoute rather than a "/*filepath" route:
	// gin refuses to hold a catch-all at the root and /healthz in the same tree.
	r.NoRoute(staticHandler.ServeFiles)

	return r
}

func healthz(c *gin.Context) {
	c.String(http.StatusOK, "ok")
}

func loggingMiddleware(logger *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// The liveness probe runs constantly and says nothing useful; logging
		// it only buries the requests that matter.
		if c.Request.URL.Path == healthPath {
			c.Next()
			return
		}

		start := time.Now()
		c.Next()
		logger.WithFields(logrus.Fields{
			"method":     c.Request.Method,
			"path":       c.Request.URL.Path,
			"host":       c.Request.Host,
			"status":     c.Writer.Status(),
			"duration":   time.Since(start).Milliseconds(),
			"user_agent": c.Request.UserAgent(),
		}).Info("Request handled")
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		b, err := strconv.ParseBool(value)
		if err != nil {
			return defaultValue
		}
		return b
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		d, err := time.ParseDuration(value)
		if err != nil {
			return defaultValue
		}
		return d
	}
	return defaultValue
}

func getEnvInt64(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return defaultValue
		}
		return n
	}
	return defaultValue
}
