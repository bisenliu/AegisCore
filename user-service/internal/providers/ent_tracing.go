package providers

import (
	"context"
	"reflect"

	entgo "entgo.io/ent"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/aegiscore/user-service/internal/persistence/ent"
)

// entTracingPlugin 在 Ent client 上安装 query 和 mutation tracing。
type entTracingPlugin struct {
	tracer trace.Tracer
}

// InstallEntClientPlugin 安装 tracing hook/interceptor；是否安装由插件构造阶段决定。
func (p entTracingPlugin) InstallEntClientPlugin(client *ent.Client) error {
	installEntQueryTracing(client, p.tracer)
	installEntMutationTracing(client, p.tracer)
	return nil
}

// installEntQueryTracing 为 Ent query/select 路径记录 ent.query span。
func installEntQueryTracing(client *ent.Client, tracer trace.Tracer) {
	if client == nil || tracer == nil {
		return
	}
	client.Intercept(ent.InterceptFunc(func(next ent.Querier) ent.Querier {
		return ent.QuerierFunc(func(ctx context.Context, query ent.Query) (ent.Value, error) {
			entity := entQueryEntity(query)
			ctx, span := tracer.Start(ctx, "ent.query",
				trace.WithAttributes(
					attribute.String("db.system", "postgresql"),
					attribute.String("ent.entity", entity),
					attribute.String("ent.operation", entQueryOperation),
				),
			)
			defer span.End()
			value, err := next.Query(ctx, query)
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, "ent query failed")
			}
			return value, err
		})
	}))
}

// installEntMutationTracing 为 Ent mutation 路径记录 ent.mutation span。
func installEntMutationTracing(client *ent.Client, tracer trace.Tracer) {
	if client == nil || tracer == nil {
		return
	}
	client.Use(func(next ent.Mutator) ent.Mutator {
		return ent.MutateFunc(func(ctx context.Context, mutation ent.Mutation) (ent.Value, error) {
			entity := entMutationEntity(mutation)
			operation := entMutationOperation(mutation)
			ctx, span := tracer.Start(ctx, "ent.mutation",
				trace.WithAttributes(
					attribute.String("db.system", "postgresql"),
					attribute.String("ent.entity", entity),
					attribute.String("ent.operation", operation),
				),
			)
			defer span.End()
			value, err := next.Mutate(ctx, mutation)
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, "ent mutation failed")
			}
			return value, err
		})
	})
}

// entMutationEntity 将 Ent mutation 类型映射为固定低基数实体标签。
func entMutationEntity(mutation ent.Mutation) string {
	if mutation == nil {
		return "unknown"
	}
	if typed, ok := mutation.(interface{ Type() string }); ok {
		return entEntityFromTypeName(typed.Type(), "")
	}
	return entEntityFromTypeName(reflect.TypeOf(mutation).String(), "Mutation")
}

// entMutationOperation 将 Ent mutation op 映射为稳定操作枚举。
func entMutationOperation(mutation ent.Mutation) string {
	if mutation == nil {
		return "unknown"
	}
	switch op := mutation.Op(); {
	case op.Is(entgo.OpCreate):
		return "create"
	case op.Is(entgo.OpUpdate | entgo.OpUpdateOne):
		return "update"
	case op.Is(entgo.OpDelete | entgo.OpDeleteOne):
		return "delete"
	default:
		return "unknown"
	}
}
