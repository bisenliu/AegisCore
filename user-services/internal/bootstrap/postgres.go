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

type PostgresParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Config    *config.Config
	Log       *zap.Logger
}

type PostgresPools struct {
	fx.Out

	UserDB   *sql.DB `name:"user_db"`
	CommonDB *sql.DB `name:"common_db"`
}

func NewPostgresPools(params PostgresParams) (PostgresPools, error) {
	userDB, err := commoninfra.NewPostgres(params.Lifecycle, params.Config, params.Log, userDBName)
	if err != nil {
		return PostgresPools{}, err
	}
	commonDB, err := commoninfra.NewPostgres(params.Lifecycle, params.Config, params.Log, commonDBName)
	if err != nil {
		_ = userDB.Close()
		return PostgresPools{}, err
	}

	return PostgresPools{UserDB: userDB, CommonDB: commonDB}, nil
}
