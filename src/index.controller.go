package src

import (
	"grape/src/common/dto/response"
	"net/http"

	"github.com/gin-gonic/gin"
)

type IndexT interface {
	Copy(c *gin.Context)
	Image(c *gin.Context)
}

type indexController struct {
	service *IndexService
}

func NewIndexController(s *IndexService) IndexT {
	return &indexController{service: s}
}

// @Summary Copy
// @Accept json
// @Produce application/json
// @Produce application/xml
// @Success 200 {object} interface{}
// @Router /copy [post]
func (c *indexController) Copy(ctx *gin.Context) {
	var body map[string]string
	ctx.ShouldBindJSON(&body)

	err := c.service.Copy(body["query"])
	response.Handler(ctx, http.StatusNoContent, gin.H{}, err)
}

// @Summary Image
// @Accept json
// @Produce application/json
// @Produce application/xml
// @Success 200 {object} interface{}
// @Router /image [get]
func (c *indexController) Image(ctx *gin.Context) {
	res := c.service.Image()
	response.Handler(ctx, http.StatusOK, res, nil)
}
