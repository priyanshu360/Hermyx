package engine

import (
	"fmt"
	"hermyx/pkg/cache"
	"hermyx/pkg/cachemanager"
	"hermyx/pkg/models"
	"hermyx/pkg/utils/logger"
	"hermyx/pkg/utils/regex"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/valyala/fasthttp"
	"gopkg.in/yaml.v3"
)

type compiledRoute struct {
	Route        *models.RouteConfig
	PathPattern  *regexp.Regexp
	IncludeRegex *regexp.Regexp
	ExcludeRegex *regexp.Regexp
}

type HermyxEngine struct {
	config         *models.HermyxConfig
	logger         *logger.Logger
	cacheManager   *cachemanager.CacheManager
	compiledRoutes []compiledRoute
}

func InstantiateHermyxEngine(configPath string) *HermyxEngine {
	var config models.HermyxConfig

	data, err := os.ReadFile(configPath)
	if err != nil {
		log.Fatalf("Unable to read the config-path %s: %v", configPath, err)
	}

	err = yaml.Unmarshal(data, &config)
	if err != nil {
		log.Fatalf("Unable to parse the config at %s: %v", configPath, err)
	}

	// Intelligent defaults
	if config.Server.Port == 0 {
		config.Server.Port = 8080
	}
	if config.Cache.Capacity == 0 {
		config.Cache.Capacity = 1000
	}
	if config.Cache.Ttl == 0 {
		config.Cache.Ttl = 300000000000
	}
	if config.Cache.MaxContentSize == 0 {
		config.Cache.MaxContentSize = 1 * 1024 * 1024
	}
	if config.Routes == nil {
		config.Routes = []models.RouteConfig{}
	}
	if config.Log == nil {
		config.Log = &models.LogConfig{
			ToStdout: true,
			Prefix:   "[Hermyx]",
			Flags:    0,
		}
	}

	logger_, err := logger.NewLogger(config.Log)
	if err != nil {
		log.Fatalf("Unable to instantiate the logger: %v", err)
	}

	cache_ := cache.NewCache(config.Cache.Capacity)
	cacheManager := cachemanager.NewCacheManager(cache_)

	engine := &HermyxEngine{
		config:       &config,
		logger:       logger_,
		cacheManager: cacheManager,
	}

	engine.compileRoutes()

	return engine
}

// Prepare compiled regexes and resolve cache configs
func (engine *HermyxEngine) compileRoutes() {
	engine.compiledRoutes = []compiledRoute{}
	for i := range engine.config.Routes {
		route := &engine.config.Routes[i]

		cr := compiledRoute{
			Route:       route,
			PathPattern: regexp.MustCompile(route.Path),
		}

		if len(route.Include) > 0 {
			cr.IncludeRegex = regex.CombinePattenrs(route.Include)
		}
		if len(route.Exclude) > 0 {
			cr.ExcludeRegex = regex.CombinePattenrs(route.Exclude)
		}

		route.Cache = engine.cacheManager.Resolve(engine.config.Cache, route.Cache)
		engine.compiledRoutes = append(engine.compiledRoutes, cr)
	}
}

func (engine *HermyxEngine) Run() {
	addr := fmt.Sprintf(":%d", engine.config.Server.Port)
	engine.logger.Info(fmt.Sprintf("Hermyx engine starting on %s...", addr))

	err := fasthttp.ListenAndServe(addr, engine.handleRequest)
	if err != nil {
		engine.logger.Error(fmt.Sprintf("Fatal server error: %v", err))
	}
}

func (engine *HermyxEngine) handleRequest(ctx *fasthttp.RequestCtx) {
	path := string(ctx.Path())
	method := strings.ToLower(string(ctx.Method()))
	engine.logger.Info(fmt.Sprintf("Incoming request - Method: %s, Path: %s", method, path))

	cr, matched := engine.matchRoute(path, method)
	if !matched {
		engine.logger.Info(fmt.Sprintf("No route matched for %s %s; proxying raw", method, path))
		if err := engine.fallbackProxy(ctx); err != nil {
			engine.logger.Error("Fallback proxy error: " + err.Error())
			ctx.Error("Fallback proxy error: "+err.Error(), fasthttp.StatusBadGateway)
		}
		return
	}

	if cr.Route.Cache.KeyConfig == nil {
		engine.logger.Error("Cache KeyConfig is nil for route " + cr.Route.Path)
		ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
		return
	}

	key := engine.cacheManager.GetKey(cr.Route.Cache.KeyConfig, ctx)
	engine.logger.Debug(fmt.Sprintf("Cache key generated: %s", key))

	if cr.Route.Cache.Enabled {
		if hit := engine.handleCache(ctx, cr, key); hit {
			return
		}
	}

	if err := engine.proxyRequest(ctx, cr); err != nil {
		engine.logger.Error(fmt.Sprintf("Proxy error for %s %s: %v", method, path, err))
		ctx.Error("Proxy error: "+err.Error(), fasthttp.StatusBadGateway)
		return
	}

	engine.cacheResponse(cr, key, ctx)
}

func (engine *HermyxEngine) matchRoute(path, method string) (*compiledRoute, bool) {
	for i := range engine.compiledRoutes {
		cr := &engine.compiledRoutes[i]

		if !cr.PathPattern.MatchString(path) {
			engine.logger.Debug(fmt.Sprintf("Route %q skipped: path pattern mismatch for %s", cr.Route.Path, path))
			continue
		}

		if cr.IncludeRegex != nil && !cr.IncludeRegex.MatchString(path) {
			engine.logger.Debug(fmt.Sprintf("Route %q skipped: include regex does not match path %s", cr.Route.Path, path))
			continue
		}

		if cr.ExcludeRegex != nil && cr.ExcludeRegex.MatchString(path) {
			engine.logger.Debug(fmt.Sprintf("Route %q skipped: exclude regex matches path %s", cr.Route.Path, path))
			continue
		}

		// Check excluded methods
		if cr.Route.Cache.KeyConfig != nil {
			for _, excludedMethod := range cr.Route.Cache.KeyConfig.ExcludeMethods {
				if strings.ToLower(excludedMethod) == method {
					engine.logger.Info(fmt.Sprintf("Request method %s excluded for route %s", method, cr.Route.Path))
					return nil, false
				}
			}
		}

		return cr, true
	}
	return nil, false
}

func (engine *HermyxEngine) handleCache(ctx *fasthttp.RequestCtx, cr *compiledRoute, key string) bool {
	res, exists := engine.cacheManager.Get(key)
	if exists {
		engine.logger.Info(fmt.Sprintf("Cache HIT for key %s (path %s)", key, string(ctx.Path())))
		ctx.SetStatusCode(fasthttp.StatusOK)
		ctx.Response.Header.Set("X-Hermyx-Cache", "HIT")
		ctx.SetBody(res.([]byte))
		return true
	}
	engine.logger.Info(fmt.Sprintf("Cache MISS for key %s (path %s)", key, string(ctx.Path())))
	return false
}

func (engine *HermyxEngine) proxyRequest(ctx *fasthttp.RequestCtx, cr *compiledRoute) error {
	target := cr.Route.Target
	addr := strings.TrimPrefix(strings.TrimPrefix(target, "http://"), "https://")
	client := &fasthttp.HostClient{Addr: addr}

	engine.logger.Info(fmt.Sprintf("Proxying request %s %s to backend %s", string(ctx.Method()), string(ctx.Path()), addr))
	return client.Do(&ctx.Request, &ctx.Response)
}

func (engine *HermyxEngine) cacheResponse(cr *compiledRoute, key string, ctx *fasthttp.RequestCtx) {
	status := ctx.Response.StatusCode()
	if status >= 200 && status < 300 {
		body := ctx.Response.Body()
		if uint64(len(body)) <= cr.Route.Cache.MaxContentSize {
			cacheTtl := cr.Route.Cache.Ttl
			engine.cacheManager.Set(key, append([]byte(nil), body...), cacheTtl)
			engine.logger.Info(fmt.Sprintf("Cached response for key %s with TTL %s", key, cacheTtl.String()))
		} else {
			engine.logger.Info(fmt.Sprintf("Response size %d exceeds max cache size %d; skipping cache for key %s", len(body), cr.Route.Cache.MaxContentSize, key))
		}
	} else {
		engine.logger.Debug(fmt.Sprintf("Not caching response for key %s due to status %d", key, status))
	}
}

func (engine *HermyxEngine) fallbackProxy(ctx *fasthttp.RequestCtx) error {
	host := string(ctx.Host())
	if host == "" {
		engine.logger.Warn("No Host header present on unmatched request; returning 404")
		ctx.Error("No route matched and no Host header present", fasthttp.StatusNotFound)
		return nil
	}

	client := &fasthttp.HostClient{Addr: host}
	engine.logger.Info(fmt.Sprintf("Fallback proxying to %s", host))
	return client.Do(&ctx.Request, &ctx.Response)
}
