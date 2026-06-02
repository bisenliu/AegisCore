package bootstrap

import (
	"database/sql"

	"github.com/aegiscore/common/config"
	commoninfra "github.com/aegiscore/common/infrastructure"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

const (
	userDBName   = "user_db"
	commonDBName = "common_db"
)

type NamedPostgresParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Config    *config.Config
	Log       *zap.Logger
}

type NamedPostgresPools struct {
	fx.Out

	UserDB   *sql.DB `name:"user_db"`
	CommonDB *sql.DB `name:"common_db"`
}

func NewPostgresPools(params NamedPostgresParams) (NamedPostgresPools, error) {
	userDB, err := commoninfra.NewPostgres(params.Lifecycle, params.Config, params.Log, userDBName)
	if err != nil {
		return NamedPostgresPools{}, err
	}
	commonDB, err := commoninfra.NewPostgres(params.Lifecycle, params.Config, params.Log, commonDBName)
	if err != nil {
		_ = userDB.Close()
		return NamedPostgresPools{}, err
	}

	return NamedPostgresPools{UserDB: userDB, CommonDB: commonDB}, nil
}
