## Context

AegisCore 当前只有 `GET /api/v1/users/:id` 一个业务 API，controller 内部通过 `strconv.ParseInt` 和私有 `validator.New(validator.WithRequiredStructEnabled())` 完成 URI 参数校验。该方式对单接口可接受，但随着后续新增 JSON body、query、form 或 URI DTO，会在每个 controller 中重复创建 validator、重复处理 binding 错误、重复拼接错误消息，并难以统一字段名、默认值和自定义规则。

外部模块 `/Users/liubisen/Desktop/sander/Project/my/go-micro-scaffold/common/pkg/validation` 的核心功能包括：

- `Validator` 封装 `go-playground/validator/v10` 和 translator，向 controller 暴露 `Verify` 与 `ValidateError`。
- Gin binding 适配器：`URIBindAdapter`、`JSONBindAdapter`、`QueryBindAdapter`、`FormBindAdapter`。
- `Validatable` 和 `Defaultable` 扩展接口，用于 DTO 自定义校验和默认值填充。
- `ValidationError` 业务校验错误，用于统一写出校验失败响应。
- `enum` 自定义校验规则，通过 DTO 字段类型实现 `IsValid() bool`。
- JSON 反序列化类型错误识别和字段显示名解析。

迁移必要性较高，但不应直接复制。收益主要是统一请求校验规范、减少 controller 重复代码、把可复用基础能力放入 `common`、让未来新增接口自动遵循 `common/response.Envelope` 和 trace-id 日志约定。直接迁移的主要问题是外部模块导入路径为 `common/...`、响应 helper 与 AegisCore 不一致、依赖全局 Gin validator 引擎、locale 注册存在错误，以及 `enum` 类型断言会 panic。

## Goals / Non-Goals

**Goals:**

- 在 `common/validation` 提供可复用、可测试、无业务耦合的请求绑定与校验组件。
- 统一 URI、query、JSON、form 的 binding 入口，并把失败映射为 AegisCore 的 `common/response.Envelope`。
- 支持字段名解析：优先 `label`，其次 `json`、`form`、`uri`、`query`，最后字段名。
- 支持 DTO 默认值钩子、自定义 `Validate() error` 钩子和安全的枚举校验规则。
- 通过 Fx provider 复用同一个 validator 实例，不在每个 controller 中重复创建。
- 保持 `user-profile-query` 现有外部行为兼容，尤其是无效用户 ID 仍返回 `invalid user id`。

**Non-Goals:**

- 不新增认证、授权、用户创建/更新/删除等业务 API。
- 不修改 Ent schema、Atlas migration、Redis 或 PostgreSQL 初始化逻辑。
- 不把 `common/config.Load` 改成全局配置校验器；请求校验和配置校验保持职责分离。
- 不强制所有现有 controller 立即迁移；先提供共享能力，再逐步替换。

## Decisions

### 1. 将校验模块放在 `common/validation`

选择：新增 `github.com/aegiscore/common/validation` 包，由 `common` 提供共享基础能力，`user-services` 注入使用。

原因：请求校验是跨服务基础能力，属于 `common` 的共享 HTTP 基础设施范围。放在 `user-services` 会导致未来服务重复实现；放在 `common/response` 会混淆响应输出和请求解析职责。

替代方案：仅在 `user-services/internal/controller` 增加 helper。该方案短期改动最小，但无法形成跨服务统一规范，后续服务仍会复制。

### 2. 使用独立 `validator.Validate` 实例，不复用 `binding.Validator.Engine()`

选择：`validation.New()` 内部创建 `validator.New(validator.WithRequiredStructEnabled())`，并只在该实例上注册 tag name、translation 和自定义规则。

原因：外部实现直接修改 Gin 全局 validator 引擎，可能被多个模块重复注册，影响所有 `ShouldBind*` 调用，且测试间存在隐藏状态。独立实例更容易单元测试，也避免全局副作用。

替代方案：继续复用 Gin 全局 binding validator。该方案能让 `ShouldBind*` 自动使用同一 validator，但增加全局副作用和重复注册风险。

### 3. Binding 和结构体验证分两步执行

选择：adapter 只负责从 Gin context 绑定到 DTO；`Validator.Bind` 在绑定后调用 `SetDefaults()`、`validate.Struct()` 和 `Validate()`。

原因：Gin 的 `ShouldBind*` 默认会触发全局 validator。为避免双重校验和全局状态，JSON 可优先使用 `ShouldBindBodyWith` 或 decoder 绑定，URI/query/form 在绑定后统一由独立 validator 执行。实现时应避免对 body 重复读取导致下游无法读取。

替代方案：完全依赖 `ShouldBindJSON` 的内置校验。该方案简单，但字段名、翻译、自定义规则和错误格式难以统一。

### 4. 错误输出兼容 `common/response.Envelope`

选择：提供 `BindOrAbort(c, dst, binder) bool` 这类 controller helper，在校验失败时调用 `response.BadRequest` 或新增支持 data 的响应 helper；首期为了兼容现有契约，默认输出 `BAD_REQUEST` 和调用方可读 message，字段级错误可作为可选 data 扩展。

原因：AegisCore 的 API 规则要求业务 API 使用 `Envelope`。现有 `response.Fail` 不包含 data，直接迁移外部模块的 `response.HandleWith(... WithData(...))` 不兼容。保持默认行为兼容可以避免影响已有客户端。

替代方案：新增 `VALIDATION_FAILED` 错误码并强制返回字段 map。该方案更表达性强，但改变错误码契约，应作为单独 API response 变更评估。

### 5. 安全实现枚举校验

选择：`ValidateEnum` 必须处理 nil、指针、非接口类型和未实现 `Enum` 的字段，返回 `false` 而不是 panic。

原因：外部实现 `fl.Field().Interface().(Enum)` 对任何未实现接口的字段都会 panic，导致 validation 失败变成 500 或 recovery。校验函数必须只返回 bool。

替代方案：要求所有 `enum` tag 只能用于实现 `Enum` 的字段。仅靠约定无法保护运行时稳定性。

### 6. 暂不把 locale 放入全局配置契约

选择：`New(Options{Locale: "zh"})` 或 Fx provider 使用默认 `zh`，并允许未来从服务配置读取 locale；不修改 `common/config.Load` 的 required/range 校验职责。

原因：当前 `config.Config` 没有 validation locale 字段。为了迁移请求校验，不应引入配置契约变更。外部代码声明支持 `en`，但只注册了中文翻译；首期应明确支持 `zh`，英文支持等完整翻译注册后再开放。

替代方案：立即新增 `validation.locale` 配置。该方案会扩大影响面，并需要补充配置主规格和样例。

## Code Review And Optimized Implementation

### 外部代码合理性审查

- `NewLocalizedValidator` 声明支持 `zh` 和 `en`，但无论 locale 是什么都调用 `zhTranslations.RegisterDefaultTranslations`，`en` 会得到不匹配或不完整的翻译。
- `binding.Validator.Engine()` 修改 Gin 全局 validator，可能被多个模块重复注册 tag name、translation 和 validation，测试和服务间存在隐藏副作用。
- `ValidateEnum` 使用强制类型断言，字段未实现 `Enum`、字段为 nil 指针或 tag 用错时会 panic。
- `translate` 中 `panic(fmt.Errorf("translator failed: %w", fe.(error)))` 再次对 `FieldError` 做错误断言，可能 panic，且校验翻译失败不应导致服务 panic。
- `RegisterTagNameFunc` 判断 `jsonName == "_"`，但 Go JSON 忽略字段惯例是 `json:"-"`。
- `label` 被编码为 `jsonName + "|" + label`，再由 `removeTopStruct` 按 `|` 拆分，字段名和翻译消息耦合，label 包含 `|` 时会产生错误。
- `removeTopStruct` 遍历 `err.Translate(trans)` 的 value，丢弃原 map key，依赖翻译字符串格式，无法稳定处理嵌套字段。
- `getFieldJSONName` 仅处理顶层字段，`json.UnmarshalTypeError.Field` 可能是嵌套路径；`reflect.TypeOf(params)` 未处理非 struct、nil、指针链。
- `JSONBindAdapter` 使用 `GetRawData` 手动读 body 并重置，对大 body 有额外内存复制；更适合使用 Gin 的 `ShouldBindBodyWith` 或直接 decoder 并受上游 body limit 控制。
- `FormBindAdapter` 使用 `ShouldBind` 会按 content-type 选择 binding，不是纯 form 语义；命名为 form 时应明确使用 form binding 或更通用地命名为 `AutoBind`。
- `Validatable` 接口定义了但 `verify` 未调用 `Validate()`，业务自定义校验能力不完整。
- `Defaultable` 在绑定后、结构体验证前调用更合适，否则默认值无法参与 `required`、`oneof`、范围等校验；外部代码在绑定后返回 true 前调用，但若 Gin 内置 validator 已先执行，则默认值无法补救 required 校验。
- 日志导入 `common/logger` 与 `common/response` 不符合 AegisCore 模块路径，也未确保 trace-id 字段通过当前 `common/logger` context API 输出。

### 优化后的 Go 代码方案

以下代码应作为实现阶段的目标结构写入 `common/validation`。它避免全局 validator 副作用，并兼容当前响应契约。具体实现可按文件拆分，示例使用完整包代码呈现：

```go
package validation

import (
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "reflect"
    "strings"

    "github.com/aegiscore/common/logger"
    "github.com/aegiscore/common/response"
    "github.com/gin-gonic/gin"
    "github.com/gin-gonic/gin/binding"
    "github.com/go-playground/locales/zh"
    ut "github.com/go-playground/universal-translator"
    "github.com/go-playground/validator/v10"
    zhTranslations "github.com/go-playground/validator/v10/translations/zh"
    "go.uber.org/fx"
    "go.uber.org/zap"
)

const (
    DefaultLocale       = "zh"
    ErrEmptyRequestBody = "请求体参数不能为空"
)

type Binder func(*gin.Context, any) error

type Defaultable interface {
    SetDefaults()
}

type Validatable interface {
    Validate() error
}

type Enum interface {
    IsValid() bool
}

type FieldError struct {
    Field   string `json:"field"`
    Message string `json:"message"`
}

type Error struct {
    Message string
    Fields  []FieldError
}

func (e *Error) Error() string {
    if e == nil {
        return ""
    }
    return e.Message
}

type Validator struct {
    validate *validator.Validate
    trans    ut.Translator
}

type Options struct {
    Locale string
}

func New(opts Options) (*Validator, error) {
    locale := opts.Locale
    if locale == "" {
        locale = DefaultLocale
    }
    if locale != DefaultLocale {
        return nil, fmt.Errorf("unsupported validation locale %q", locale)
    }

    validate := validator.New(validator.WithRequiredStructEnabled())
    validate.RegisterTagNameFunc(fieldName)

    zhLocale := zh.New()
    uni := ut.New(zhLocale, zhLocale)
    trans, ok := uni.GetTranslator(locale)
    if !ok {
        return nil, fmt.Errorf("validation translator %q is unavailable", locale)
    }
    if err := zhTranslations.RegisterDefaultTranslations(validate, trans); err != nil {
        return nil, fmt.Errorf("register validation translations: %w", err)
    }
    if err := validate.RegisterValidation("enum", validateEnum); err != nil {
        return nil, fmt.Errorf("register enum validation: %w", err)
    }
    if err := validate.RegisterTranslation("enum", trans, func(t ut.Translator) error {
        return t.Add("enum", "{0}不合法", false)
    }, func(t ut.Translator, fe validator.FieldError) string {
        msg, err := t.T(fe.Tag(), fe.Field())
        if err != nil {
            return fe.Field() + "不合法"
        }
        return msg
    }); err != nil {
        return nil, fmt.Errorf("register enum translation: %w", err)
    }
    return &Validator{validate: validate, trans: trans}, nil
}

func NewDefault() (*Validator, error) {
    return New(Options{Locale: DefaultLocale})
}

var Module = fx.Module("validation", fx.Provide(NewDefault))

func (v *Validator) Validate(dst any) error {
    if d, ok := dst.(Defaultable); ok {
        d.SetDefaults()
    }
    if err := v.validate.Struct(dst); err != nil {
        return v.normalizeError(dst, err)
    }
    if custom, ok := dst.(Validatable); ok {
        if err := custom.Validate(); err != nil {
            return err
        }
    }
    return nil
}

func (v *Validator) Bind(c *gin.Context, dst any, binder Binder) error {
    if err := binder(c, dst); err != nil {
        return v.normalizeError(dst, err)
    }
    return v.Validate(dst)
}

func (v *Validator) BindOrAbort(c *gin.Context, dst any, binder Binder) bool {
    if err := v.Bind(c, dst, binder); err != nil {
        logger.Warn(c.Request.Context(), "invalid request", zap.Error(err), zap.String("path", c.Request.URL.Path))
        response.BadRequest(c, publicMessage(err))
        c.Abort()
        return false
    }
    return true
}

func (v *Validator) normalizeError(dst any, err error) error {
    if err == nil {
        return nil
    }
    var validationErrors validator.ValidationErrors
    if errors.As(err, &validationErrors) {
        fields := make([]FieldError, 0, len(validationErrors))
        for _, fieldErr := range validationErrors {
            fields = append(fields, FieldError{Field: fieldErr.Field(), Message: fieldErr.Translate(v.trans)})
        }
        return &Error{Message: "请求参数验证失败", Fields: fields}
    }
    var typeErr *json.UnmarshalTypeError
    if errors.As(err, &typeErr) {
        field := displayName(dst, typeErr.Field)
        return &Error{Message: fmt.Sprintf("%s字段类型不正确，应为%s类型", field, expectedType(typeErr.Type))}
    }
    if errors.Is(err, io.EOF) {
        return &Error{Message: ErrEmptyRequestBody}
    }
    return err
}

func URIBinder(c *gin.Context, dst any) error {
    return c.ShouldBindUri(dst)
}

func QueryBinder(c *gin.Context, dst any) error {
    return c.ShouldBindQuery(dst)
}

func JSONBinder(c *gin.Context, dst any) error {
    if c.Request.Body == nil || c.Request.ContentLength == 0 {
        return &Error{Message: ErrEmptyRequestBody}
    }
    return c.ShouldBindBodyWith(dst, binding.JSON)
}

func FormBinder(c *gin.Context, dst any) error {
    return c.ShouldBindWith(dst, binding.Form)
}

func validateEnum(fl validator.FieldLevel) bool {
    value := fl.Field()
    if !value.IsValid() {
        return false
    }
    if value.Kind() == reflect.Ptr {
        if value.IsNil() {
            return false
        }
        value = value.Elem()
    }
    enum, ok := value.Interface().(Enum)
    return ok && enum.IsValid()
}

func fieldName(fld reflect.StructField) string {
    if label := fld.Tag.Get("label"); label != "" {
        return label
    }
    for _, tag := range []string{"json", "form", "uri", "query"} {
        name := strings.SplitN(fld.Tag.Get(tag), ",", 2)[0]
        if name == "-" {
            return ""
        }
        if name != "" {
            return name
        }
    }
    return fld.Name
}

func publicMessage(err error) string {
    var validationErr *Error
    if errors.As(err, &validationErr) && validationErr.Message != "" {
        return validationErr.Message
    }
    return err.Error()
}

func expectedType(t reflect.Type) string {
    if t == nil {
        return "正确"
    }
    switch t.Kind() {
    case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
        return "整数"
    case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
        return "正整数"
    case reflect.Float32, reflect.Float64:
        return "浮点数"
    case reflect.Bool:
        return "布尔"
    case reflect.String:
        return "字符串"
    case reflect.Array, reflect.Slice:
        return "数组"
    case reflect.Map:
        return "映射"
    default:
        return t.String()
    }
}

func displayName(dst any, path string) string {
    if path == "" {
        return "参数"
    }
    typ := reflect.TypeOf(dst)
    for typ.Kind() == reflect.Ptr {
        typ = typ.Elem()
    }
    if typ.Kind() != reflect.Struct {
        return path
    }
    current := typ
    parts := strings.Split(path, ".")
    for _, part := range parts {
        found := false
        for i := 0; i < current.NumField(); i++ {
            field := current.Field(i)
            name := strings.SplitN(field.Tag.Get("json"), ",", 2)[0]
            if name == part || field.Name == part {
                if label := field.Tag.Get("label"); label != "" {
                    return label
                }
                if name != "" && name != "-" {
                    return name
                }
                if field.Type.Kind() == reflect.Struct {
                    current = field.Type
                    found = true
                    break
                }
                return field.Name
            }
        }
        if !found {
            return part
        }
    }
    return path
}
```

## Integration Plan

1. 在 `common/validation` 增加上述实现和单元测试，覆盖 URI/query/JSON 绑定、空 body、类型错误、字段 tag 显示名、`Defaultable`、`Validatable`、`enum` 正常与错误使用场景。
2. 在 `common/go.mod` 将 `validator/v10`、`locales`、`universal-translator` 调整为直接依赖；运行 `go mod tidy` 保持模块依赖准确。
3. 将 `validation.Module` 纳入共享基础设施装配，或在 `user-services/internal/bootstrap` 的 Fx module 中显式引入 `validation.Module`。推荐先在用户服务 module 显式引入，避免让所有使用 `common/infrastructure.Module` 的服务都无条件获得 HTTP validation 依赖。
4. 更新 `UserController` 构造函数注入 `*validation.Validator`，把 `validator.New(...)` 字段替换为共享 validator。`GetByID` 可定义 URI DTO：

```go
type getUserURI struct {
    ID int64 `uri:"id" validate:"required,gt=0" label:"用户ID"`
}
```

5. 为了保持现有 `user-profile-query` 契约，controller 在共享校验失败后仍输出 `invalid user id`，而不是暴露中文字段级消息。后续新增接口可直接使用 `BindOrAbort` 默认消息。
6. 运行 `go test ./...` 分别验证 `common` 和 `user-services` 模块。

## Risks / Trade-offs

- [Risk] 直接使用 `ShouldBindUri`、`ShouldBindQuery` 可能仍触发 Gin 全局 validator，导致与独立 validator 双重校验。→ Mitigation：实现阶段评估 Gin binding 行为；必要时使用只绑定不校验的 decoder/form mapper，或接受双重校验但只对外处理统一 validator 的错误。
- [Risk] 字段级错误 data 需要响应契约扩展，目前 `response.Fail` 不支持 data。→ Mitigation：首期默认只返回 message，避免改变 API 契约；如需要字段 map，单独修改 `api-response-contract`。
- [Risk] 本地化翻译增加依赖体积和初始化复杂度。→ Mitigation：首期只支持 `zh`，初始化失败直接让 Fx 启动失败，避免运行期不确定行为。
- [Risk] 迁移 `UserController` 可能改变 `invalid user id` 错误消息。→ Mitigation：对 `GET /api/v1/users/abc` 和 `/0` 增加/保留测试，保证兼容。
- [Risk] 把 validation module 加入 `common/infrastructure.Module` 会扩大所有服务依赖。→ Mitigation：优先作为独立 `validation.Module`，由需要 HTTP 请求校验的服务显式引入。
