package bootstrap

import (
	"database/sql"

	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/datastorefx"
	"github.com/aegiscore/common/runtime/resources"
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
	dbs, err := datastorefx.NewPostgresPools(
		params.Lifecycle,
		params.Config,
		params.Log,
		resources.NameUserDB,
		resources.NameCommonDB,
	)
	if err != nil {
		return NamedPostgresPools{}, err
	}

	return NamedPostgresPools{
		UserDB:   dbs[resources.NameUserDB],
		CommonDB: dbs[resources.NameCommonDB],
	}, nil
}
