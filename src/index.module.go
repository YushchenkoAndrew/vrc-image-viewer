package src

import (
	m "grape/src/common/module"
	"grape/src/common/service"

	"github.com/gin-gonic/gin"
)

type indexModule struct {
	*m.Module[IndexT]
}

func NewIndexModule(route *gin.RouterGroup, modules []m.ModuleT, s *service.CommonService) m.ModuleT {
	return &indexModule{
		Module: &m.Module[IndexT]{
			Route:      route,
			Controller: NewIndexController(NewIndexService(s)),
			Modules:    modules,
		},
	}
}

func (c *indexModule) Init() {
	// c.Route.GET("/", c.Controller.Index)
	c.Route.GET("/images", c.Controller.Image)
	c.Route.POST("/copy", c.Controller.Copy)

	c.Module.Init()
}
