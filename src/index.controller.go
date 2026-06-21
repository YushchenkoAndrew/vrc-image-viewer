package src

import (
	"grape/src/common/dto/request"
	"grape/src/common/dto/response"
	"net/http"

	"github.com/gin-gonic/gin"
)

type IndexT interface {
	Copy(c *gin.Context)
	FindAll(c *gin.Context)
	Delete(c *gin.Context)
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

// @Summary FindAll
// @Accept json
// @Produce application/json
// @Produce application/xml
// @Success 200 {object} interface{}
// @Router /images [get]
func (c *indexController) FindAll(ctx *gin.Context) {
	dto := request.NewPageDto()

	if err := ctx.ShouldBindQuery(&dto); err != nil {
		response.BadRequest(ctx, err)
		return
	}

	res := c.service.FindAll(dto)
	response.Handler(ctx, http.StatusOK, res, nil)
}

// @Summary Delete
// @Accept json
// @Produce application/json
// @Produce application/xml
// @Param id path string true "Image mda5 id"
// @Success 200 {object} interface{}
// @Router /images/:id [delete]
func (c *indexController) Delete(ctx *gin.Context) {
	res := c.service.Delete(ctx.Param("id"))
	response.Handler(ctx, http.StatusOK, res, nil)
}
