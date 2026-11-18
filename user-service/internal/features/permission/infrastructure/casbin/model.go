package casbin

import (
	_ "embed"

	"github.com/casbin/casbin/v3/model"
)

//go:embed model.conf
var modelText string

// newModel 创建基于“用户-角色-功能”的权限判定模型。
func newModel() (model.Model, error) {
	return model.NewModelFromString(modelText)
}
