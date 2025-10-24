package engine

import (
	"fmt"
	"hermyx/pkg/models"
	"hermyx/pkg/ratelimit"
	"math"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"hermyx/pkg/utils/fs"
	"hermyx/pkg/utils/regex"

	"github.com/valyala/fasthttp"
)

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

		// Resolve and store rate limit config (same pattern as cache)
		if engine.rateLimitManager != nil {
			route.RateLimit = engine.rateLimitManager.Resolve(engine.config.RateLimit, route.RateLimit)
		}

		engine.compiledRoutes = append(engine.compiledRoutes, cr)
	}
}

func (engine *HermyxEngine) Run() {
	addr := fmt.Sprintf(":%d", engine.config.Server.Port)
	engine.logger.Info(fmt.Sprintf("Hermyx engine starting on %s...", addr))

	// Wrap handleRequest with rate limit middleware
	handler := engine.rateLimitMiddleware(engine.handleRequest)

	server := &fasthttp.Server{
		Handler:          handler,
		MaxConnsPerIP:    0,
		DisableKeepalive: false,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := server.ListenAndServe(addr); err != nil {
			engine.logger.Error(fmt.Sprintf("Fatal server error: %v", err))
			os.Exit(1)
		}
	}()

	<-stop
	engine.logger.Info("Shutting down server...")
	if err := server.Shutdown(); err != nil {
		engine.logger.Error(fmt.Sprintf("Server shutdown error: %v", err))
	}
	if cerr := engine.cleanup(); cerr != nil {
		engine.logger.Error(fmt.Sprintf("Cleanup error: %v", cerr))
	}
}

func (engine *HermyxEngine) rateLimitMiddleware(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		if engine.rateLimitManager == nil {
			next(ctx)
			return
		}

		path := string(ctx.Path())
		method := strings.ToLower(string(ctx.Method()))

		// Use matchRouteForRateLimit to ensure rate limiting works independently of cache ExcludeMethods
		cr, matched := engine.matchRouteForRateLimit(path, method)
		var config *models.RateLimitConfig

		if matched {
			config = cr.Route.RateLimit
		} else {
			config = engine.config.RateLimit
		}

		result := engine.rateLimitManager.Check(ctx, config)
		engine.logger.Debug(fmt.Sprintf("Rate limit check: allowed=%v, remaining=%d, limit=%d",
			result.Allowed, result.Remaining, result.Limit))

		if !result.Allowed {
			engine.handleRateLimitExceeded(ctx, result, config)
			return
		}

		next(ctx)
		engine.rateLimitManager.SetHeaders(ctx, result, config)
		engine.logger.Debug("Rate limit headers set")
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

	if cr.Route.Cache.Enabled && cr.Route.Cache.KeyConfig == nil {
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

func (engine *HermyxEngine) handleRateLimitExceeded(ctx *fasthttp.RequestCtx, result *ratelimit.RateLimitResult, config *models.RateLimitConfig) {
	statusCode := fasthttp.StatusTooManyRequests
	message := "Rate limit exceeded. Please try again later."

	if config != nil {
		if config.StatusCode != nil {
			statusCode = *config.StatusCode
		}
		if config.Message != "" {
			message = config.Message
		}
	}

	engine.logger.Warn(fmt.Sprintf("Rate limit exceeded for key: %s", result.Key))

	engine.rateLimitManager.SetHeaders(ctx, result, config)

	// Calculate Retry-After header with ceiling and clamp to zero
	secs := max(int(math.Ceil(time.Until(result.ResetTime).Seconds())), 0)
	ctx.Response.Header.Set("Retry-After", fmt.Sprintf("%d", secs))

	ctx.SetStatusCode(statusCode)
	ctx.SetBodyString(message)
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

// matchRouteForRateLimit matches routes for rate limiting purposes without considering cache ExcludeMethods
func (engine *HermyxEngine) matchRouteForRateLimit(path, method string) (*compiledRoute, bool) {
	for i := range engine.compiledRoutes {
		cr := &engine.compiledRoutes[i]

		if !cr.PathPattern.MatchString(path) {
			engine.logger.Debug(fmt.Sprintf("Rate limit route %q skipped: path pattern mismatch for %s", cr.Route.Path, path))
			continue
		}

		if cr.IncludeRegex != nil && !cr.IncludeRegex.MatchString(path) {
			engine.logger.Debug(fmt.Sprintf("Rate limit route %q skipped: include regex does not match path %s", cr.Route.Path, path))
			continue
		}

		if cr.ExcludeRegex != nil && cr.ExcludeRegex.MatchString(path) {
			engine.logger.Debug(fmt.Sprintf("Rate limit route %q skipped: exclude regex matches path %s", cr.Route.Path, path))
			continue
		}

		// Note: We intentionally do NOT check ExcludeMethods here
		// Rate limiting should work independently of cache configuration
		engine.logger.Debug(fmt.Sprintf("Rate limit route %q matched for %s %s", cr.Route.Path, method, path))
		return cr, true
	}
	return nil, false
}

func (engine *HermyxEngine) handleCache(ctx *fasthttp.RequestCtx, cr *compiledRoute, key string) bool {
	res, exists, err := engine.cacheManager.Get(key)
	if err != nil {
		engine.logger.Error(fmt.Sprintf("Error while accessing the cache: %s", err.Error()))
		return false
	}

	if exists {
		engine.logger.Info(fmt.Sprintf("Cache HIT for key %s (path %s)", key, string(ctx.Path())))
		ctx.SetStatusCode(fasthttp.StatusOK)
		ctx.Response.Header.Set("X-Hermyx-Cache", "HIT")
		ctx.SetBody(res)
		return true
	}
	engine.logger.Info(fmt.Sprintf("Cache MISS for key %s (path %s)", key, string(ctx.Path())))
	return false
}

func (engine *HermyxEngine) proxyRequest(ctx *fasthttp.RequestCtx, cr *compiledRoute) error {
	target := cr.Route.Target
	client := engine.getClientForTarget(target)

	engine.logger.Info(fmt.Sprintf("Proxying request %s %s to backend %s", string(ctx.Method()), string(ctx.Path()), target))
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

	client := engine.getClientForTarget(host)
	engine.logger.Info(fmt.Sprintf("Fallback proxying to %s", host))
	return client.Do(&ctx.Request, &ctx.Response)
}

func (engine *HermyxEngine) storePid() error {
	engine.logger.Info("Storing program id information...")

	storageDir := engine.config.Storage.Path

	path := filepath.Join(storageDir, "hermyx.pid")

	err := fs.EnsureDir(storageDir)
	if err != nil {
		engine.logger.Error(fmt.Sprintf("Unable to create program storage path due to %v", err))
		return err
	}

	err = os.WriteFile(path, []byte(fmt.Sprintf("%d", engine.pid)), 0o644)
	if err != nil {
		engine.logger.Error(fmt.Sprintf("Unable to store program id due to %v", err))
		return err
	}

	engine.logger.Info(fmt.Sprintf("Stored program id information at %s", path))
	return nil
}

func (engine *HermyxEngine) cleanup() error {
	var err error = nil

	// Close rate limit manager
	if engine.rateLimitManager != nil {
		if closeErr := engine.rateLimitManager.Close(); closeErr != nil {
			engine.logger.Error(fmt.Sprintf("Failed to close rate limit manager: %v", closeErr))
		}
		engine.logger.Info("Rate limit manager closed")
	}

	err = engine.cacheManager.Close()
	if err != nil {
		engine.logger.Error(fmt.Sprintf("Failed to close the cache due to: %v", err))
	}
	engine.logger.Info("Cache closed")

	pidFile := filepath.Join(engine.config.Storage.Path, "hermyx.pid")
	err = os.Remove(pidFile)
	if err != nil {
		engine.logger.Error(fmt.Sprintf("Failed to close PID file: %v", err))
	}
	engine.logger.Info("PID file removed.")
	return err
}
