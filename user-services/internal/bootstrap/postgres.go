package bootstrap

import (
	"database/sql"

	"github.com/aegiscore/common/runtime/config"
	commoninfra "github.com/aegiscore/common/runtime/infrastructure"
	"go.uber.org/fx"
	"go.uber.org/zap"
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

func ProvidePostgresPools(params NamedPostgresParams) (NamedPostgresPools, error) {
	userDB, err := commoninfra.NewPostgres(params.Lifecycle, params.Config, params.Log, commoninfra.NameUserDB)
	if err != nil {
		return NamedPostgresPools{}, err
	}
	commonDB, err := commoninfra.NewPostgres(params.Lifecycle, params.Config, params.Log, commoninfra.NameCommonDB)
	if err != nil {
		_ = userDB.Close()
		return NamedPostgresPools{}, err
	}

	return NamedPostgresPools{UserDB: userDB, CommonDB: commonDB}, nil
}
