package bootstrap

import (
	"context"
	"database/sql"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/aegiscore/common/logger"
	"github.com/aegiscore/user-services/ent"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type NamedEntClientParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Log       *zap.Logger
	UserDB    *sql.DB `name:"user_db"`
	CommonDB  *sql.DB `name:"common_db"`
}

type NamedEntClients struct {
	fx.Out

	UserClient   *ent.Client `name:"user_db"`
	CommonClient *ent.Client `name:"common_db"`
}

func ProvideEntClients(params NamedEntClientParams) NamedEntClients {
	userClient := newEntClient(params.UserDB)
	commonClient := newEntClient(params.CommonDB)

	params.Lifecycle.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			logger.WithContext(params.Log, ctx).Info("closing ent clients")
			userErr := userClient.Close()
			commonErr := commonClient.Close()
			if userErr != nil {
				return userErr
			}
			return commonErr
		},
	})

	return NamedEntClients{UserClient: userClient, CommonClient: commonClient}
}

func newEntClient(db *sql.DB) *ent.Client {
	driver := entsql.OpenDB(dialect.Postgres, db)
	return ent.NewClient(ent.Driver(driver))
}
