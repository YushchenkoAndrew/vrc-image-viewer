package request

type SettingsCreateDto struct {
	Settings string `json:"settings" xml:"settings" binding:"required"`
}
