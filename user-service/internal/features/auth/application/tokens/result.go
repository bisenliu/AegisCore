package tokens

// TokenResult 是认证流程的 transport-neutral token 载荷。
type TokenResult struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	ExpiresIn    int64
}
