package providers

import (
	"database/sql"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/datastore"
	"github.com/aegiscore/common/runtime/resources"
)

// NamedPostgresParams 包含供应用户服务 PostgreSQL 连接池所需的 Fx 输入。
type NamedPostgresParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Config    *config.Config
	Log       *zap.Logger
}

// NamedPostgresPools 包含 user 和 common PostgreSQL 连接池的 Fx 输出。
type NamedPostgresPools struct {
	fx.Out

	UserDB   *sql.DB `name:"user_db"`
	CommonDB *sql.DB `name:"common_db"`
}

// ProvidePostgresPools 供应 user-service 所需的具名 PostgreSQL 连接池。
func ProvidePostgresPools(params NamedPostgresParams) (NamedPostgresPools, error) {
	dbs, err := datastore.NewPostgresPools(
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
