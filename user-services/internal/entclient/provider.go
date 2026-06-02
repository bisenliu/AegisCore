package entclient

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

type ClientParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Log       *zap.Logger
	UserDB    *sql.DB `name:"user_db"`
	CommonDB  *sql.DB `name:"common_db"`
}

type NamedClients struct {
	fx.Out

	UserClient   *ent.Client `name:"user_db"`
	CommonClient *ent.Client `name:"common_db"`
}

func NewNamedClients(params ClientParams) NamedClients {
	userClient := newClient(params.UserDB)
	commonClient := newClient(params.CommonDB)

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

	return NamedClients{UserClient: userClient, CommonClient: commonClient}
}

func newClient(db *sql.DB) *ent.Client {
	driver := entsql.OpenDB(dialect.Postgres, db)
	return ent.NewClient(ent.Driver(driver))
}
