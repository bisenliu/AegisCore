package bootstrap

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/aegiscore/common/runtime/logger"
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
			return closeEntClients(userClient.Close, commonClient.Close)
		},
	})

	return NamedEntClients{UserClient: userClient, CommonClient: commonClient}
}

func newEntClient(db *sql.DB) *ent.Client {
	driver := entsql.OpenDB(dialect.Postgres, db)
	return ent.NewClient(ent.Driver(nonClosingEntDriver{Driver: driver}))
}

type nonClosingEntDriver struct {
	dialect.Driver
}

func (d nonClosingEntDriver) Close() error {
	return nil
}

func closeEntClients(closeUser, closeCommon func() error) error {
	userErr := closeUser()
	commonErr := closeCommon()

	return errors.Join(
		wrapEntCloseError("user_db", userErr),
		wrapEntCloseError("common_db", commonErr),
	)
}

func wrapEntCloseError(name string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("close %s ent client: %w", name, err)
}
