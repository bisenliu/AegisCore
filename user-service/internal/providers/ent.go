package providers

import (
	"context"
	"database/sql"
	"fmt"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/logger"
	"github.com/aegiscore/user-service/ent"
)

// NamedEntClientParams 包含由具名 SQL 连接池支撑 Ent client 所需的 Fx 输入。
type NamedEntClientParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Log       *zap.Logger
	UserDB    *sql.DB `name:"user_db"`
}

// NamedEntClients 包含绑定用户服务数据库的 Ent client Fx 输出。
type NamedEntClients struct {
	fx.Out

	UserClient *ent.Client `name:"user_db"`
}

// ProvideEntClients 将具名 SQL 连接池包装为 Ent client，并注册 Ent client 关闭 hook。
func ProvideEntClients(params NamedEntClientParams) NamedEntClients {
	userClient := newEntClient(params.UserDB)

	params.Lifecycle.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			logger.WithContext(ctx, params.Log).Info("closing ent clients")
			return closeEntClient("user_db", userClient.Close)
		},
	})

	return NamedEntClients{UserClient: userClient}
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

func closeEntClient(name string, closeClient func() error) error {
	if err := closeClient(); err != nil {
		return fmt.Errorf("close %s ent client: %w", name, err)
	}
	return nil
}
