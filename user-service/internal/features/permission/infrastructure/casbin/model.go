package casbin

import (
	_ "embed"

	"github.com/casbin/casbin/v3/model"
)

//go:embed model.conf
var modelText string

func newModel() (model.Model, error) {
	return model.NewModelFromString(modelText)
}
