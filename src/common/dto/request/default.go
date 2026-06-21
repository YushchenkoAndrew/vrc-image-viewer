package request

import "github.com/jinzhu/copier"

func NewRequest[Dto any](dst Dto, src ...Dto) Dto {
	for _, init := range src {
		copier.CopyWithOption(&dst, init, copier.Option{IgnoreEmpty: true})
	}
	return dst
}
