package app

import (
	"github.com/example/sfree/api-go/internal/config"
	"github.com/example/sfree/api-go/internal/db"
	"github.com/example/sfree/api-go/internal/observability"
	"github.com/example/sfree/api-go/internal/ratelimit"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

func SetupRouter(m *db.Mongo, cfg *config.Config) (*gin.Engine, error) {
	return setupRouter(m, cfg, routerSetupOptions{})
}

type routerSetupOptions struct {
	constructors routerDependencyConstructors
}

func setupRouter(m *db.Mongo, cfg *config.Config, opts routerSetupOptions) (*gin.Engine, error) {
	router := gin.New()

	registerMiddleware(router, cfg)
	registerProbeRoutes(router, m)

	deps, err := newRouterDependencies(m, cfg, opts.constructors)
	if err != nil {
		return nil, err
	}
	registerRESTRoutes(router, cfg, deps)
	registerPublicShareRoutes(router, deps)
	registerDocsMetricsRoutes(router)
	registerS3Routes(router, cfg, deps)

	return router, nil
}

func registerMiddleware(router *gin.Engine, cfg *config.Config) {
	corsConfig := cors.Config{
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "Authorization"},
		AllowCredentials: true,
	}
	if cfg != nil && cfg.FrontendURL != "" {
		corsConfig.AllowOrigins = []string{cfg.FrontendURL}
	} else {
		corsConfig.AllowAllOrigins = true
		corsConfig.AllowCredentials = false
	}
	router.Use(cors.New(corsConfig))
	router.Use(gin.Recovery())
	router.Use(observability.Middleware())
	router.Use(otelgin.Middleware("sfree-api"))

	rlCfg := ratelimit.DefaultConfig()
	if cfg != nil {
		if cfg.RateLimit.PerIP > 0 {
			rlCfg.PerIPReqsPerMin = cfg.RateLimit.PerIP
		}
		if cfg.RateLimit.PerKey > 0 {
			rlCfg.PerKeyReqsPerMin = cfg.RateLimit.PerKey
		}
	}
	router.Use(ratelimit.Middleware(rlCfg))
}
