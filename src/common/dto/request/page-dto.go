package request

type PageDto struct {
	Page      int    `form:"page,default=1" binding:"gte=1" example:"1"`
	Take      int    `form:"take,default=32" binding:"gte=1" example:"20"`
	SortBy    string `form:"sort_by,default=created_at" example:"name"`
	Direction string `form:"direction,default=desc" binding:"oneof=asc desc" example:"asc"`
}

func (c *PageDto) Offset() int {
	return (c.Page - 1) * c.Take
}

func (c *PageDto) Limit() int {
	return c.Take
}

// TODO: Make that as T
func (c *PageDto) Paginate(data []string) []string {
	start := max(0, min(c.Offset(), len(data)))
	end := max(start, min(start+c.Take, len(data)))
	return data[start:end]
}

func NewPageDto(init ...*PageDto) *PageDto {
	return NewRequest(&PageDto{}, init...)
}
