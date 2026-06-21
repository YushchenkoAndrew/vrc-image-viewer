package main

import (
	"grape/src"
	"grape/src/common/config"
	"grape/src/common/dto/response"
	"grape/src/common/middleware"
	m "grape/src/common/module"
	"grape/src/common/service"
	"grape/src/swagger"
	"net/http"
	"path/filepath"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
)

// @title Gin API
// @version 3.0
// @description Gin API for my various projects

// @contact.name API Author
// @contact.email AndrewYushchenko@gmail.com

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization

// @BasePath /grape
func main() {
	cfg := config.NewConfig("configs/", "config", "yaml")

	service := &service.CommonService{
		Config: cfg,
	}

	r := gin.Default()
	r.Use(cors.Default())
	rg := r.Group(cfg.Server.Prefix, middleware.GetMiddleware(service).Default())
	module := src.NewIndexModule(rg, []m.ModuleT{
		swagger.NewSwaggerRouter(rg),
	}, service)

	module.Init()

	// Serve the static files from the "build" directory
	r.Use(static.Serve("/", static.LocalFile(filepath.Join(cfg.Vrchat.Dst, ".."), true)))

	r.NoRoute(func(ctx *gin.Context) { response.ThrowErr(ctx, http.StatusNotFound, "Resource not found") })
	r.Run(cfg.Server.Address)
}
