type server struct {
	settings      *Settings
	template      *FlexTemplate // ✅追加
	channelSecret string
	accessToken   string
}
