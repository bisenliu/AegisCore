package password

// NewService 构造使用固定 bcrypt 策略的密码服务。
func NewService() (*Service, error) {
	return &Service{cost: defaultBcryptCost}, nil
}
