package middleware

import (
	"grape/src/common/config"
	"grape/src/common/service"
)

type Middleware struct {
	config *config.Config
}

var middleware *Middleware

func GetMiddleware(s *service.CommonService) *Middleware {
	if middleware != nil || s == nil {
		return middleware
	}

	middleware = &Middleware{config: s.Config}
	return middleware
}
