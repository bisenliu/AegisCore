package entclient

import (
	"context"
	"database/sql"
	"log/slog"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/aegiscore/user-services/ent"
	"go.uber.org/fx"
)

type Params struct {
	fx.In

	Lifecycle fx.Lifecycle
	Log       *slog.Logger
	UserDB    *sql.DB `name:"user_db"`
	CommonDB  *sql.DB `name:"common_db"`
}

type Clients struct {
	fx.Out

	UserClient   *ent.Client `name:"user_db"`
	CommonClient *ent.Client `name:"common_db"`
}

func NewClients(params Params) Clients {
	userClient := newClient(params.UserDB)
	commonClient := newClient(params.CommonDB)

	params.Lifecycle.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			params.Log.InfoContext(ctx, "closing ent clients")
			userErr := userClient.Close()
			commonErr := commonClient.Close()
			if userErr != nil {
				return userErr
			}
			return commonErr
		},
	})

	return Clients{UserClient: userClient, CommonClient: commonClient}
}

func newClient(db *sql.DB) *ent.Client {
	driver := entsql.OpenDB(dialect.Postgres, db)
	return ent.NewClient(ent.Driver(driver))
}
