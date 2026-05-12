package v1

// RefreshRequest 刷新接口请求载荷。
//
// RefreshToken 是服务端颁发的不透明字符串，客户端只能原样保存与回传。
type RefreshRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

// RefreshResponseData 刷新接口响应载荷。
//
// 每次刷新都会轮换 RefreshToken，客户端必须用响应中的新值覆盖旧值。
type RefreshResponseData struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    int64  `json:"expiresIn"`
}

type RefreshResponse struct {
	Response
	Data RefreshResponseData
}

// LogoutRequest 登出接口请求载荷。
//
// 登出只依赖 RefreshToken 定位当前会话，不要求 access token 仍有效。
type LogoutRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}
