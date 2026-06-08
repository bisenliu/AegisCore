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

// NamedEntClientParams 包含由具名 SQL 连接池支撑 Ent client 所需的 Fx 输入。
type NamedEntClientParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Log       *zap.Logger
	UserDB    *sql.DB `name:"user_db"`
	CommonDB  *sql.DB `name:"common_db"`
}

// NamedEntClients 包含绑定 user 和 common 数据库的 Ent client Fx 输出。
type NamedEntClients struct {
	fx.Out

	UserClient   *ent.Client `name:"user_db"`
	CommonClient *ent.Client `name:"common_db"`
}

// ProvideEntClients 将具名 SQL 连接池包装为 Ent client，并注册 Ent client 关闭 hook。
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
	// SQL 连接池由 datastore 生命周期 hook 持有，Ent client 不应独立关闭它们。
	driver := entsql.OpenDB(dialect.Postgres, db)
	return ent.NewClient(ent.Driver(nonClosingEntDriver{Driver: driver}))
}

type nonClosingEntDriver struct {
	dialect.Driver
}

func (d nonClosingEntDriver) Close() error {
	// Close 有意保持为空操作，避免重复关闭由 Fx datastore provider 管理的 SQL 连接池。
	return nil
}

func closeEntClients(closeUser, closeCommon func() error) error {
	userErr := closeUser()
	commonErr := closeCommon()

	// 聚合具名关闭错误，避免某个 client 失败掩盖另一个关闭失败。
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
