package redis

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"uuid"

	"go.uber.org/zap"

	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
)

const policyRefreshSchemaVersion = 1

const (
	policyRefreshKindPolicyChanged   = "policy_changed"
	policyRefreshKindUserRoleChanged = "user_role_changed"
)

// PolicyRefreshMessage 是版本化的 RBAC policy Redis Pub/Sub 刷新 hint。
type PolicyRefreshMessage struct {
	SchemaVersion    int        `json:"schema_version"`
	PolicyRevision   *int64     `json:"policy_revision,omitempty"`
	UserRoleRevision *int64     `json:"user_role_revision,omitempty"`
	Kind             string     `json:"kind"`
	Reason           string     `json:"reason"`
	InstanceID       string     `json:"publisher_instance_id"`
	UserID           *uuid.UUID `json:"user_id,omitempty"`
	RoleID           *uuid.UUID `json:"role_id,omitempty"`
	PermissionID     *uuid.UUID `json:"permission_id,omitempty"`
}

// newPolicyRefreshMessage 组装一条权限变更的广播通知，包含最新版本号以及发生变更的具体类型。
func newPolicyRefreshMessage(revision int64, change permissionapplication.PolicyChange, instanceID string) PolicyRefreshMessage {
	message := PolicyRefreshMessage{
		SchemaVersion: policyRefreshSchemaVersion,
		Reason:        change.ReasonText(),
		InstanceID:    instanceID,
		UserID:        optionalUUID(change.UserID),
		RoleID:        optionalUUID(change.RoleID),
		PermissionID:  optionalUUID(change.PermissionID),
	}
	if change.Kind == permissionapplication.PolicyChangeKindUserRole {
		message.Kind = policyRefreshKindUserRoleChanged
		message.UserRoleRevision = int64Pointer(revision)
		return message
	}
	message.Kind = policyRefreshKindPolicyChanged
	message.PolicyRevision = int64Pointer(revision)
	return message
}

// encodePolicyRefreshMessage 将权限变更通知打包为可以在网络中广播传输的格式；内容损坏时报错。
func encodePolicyRefreshMessage(message PolicyRefreshMessage) (string, error) {
	if err := validatePolicyRefreshMessage(message); err != nil {
		return "", err
	}
	payload, err := json.Marshal(message)
	if err != nil {
		return "", fmt.Errorf("marshal rbac policy refresh message: %w", err)
	}
	return string(payload), nil
}

// decodePolicyRefreshMessage 将网络广播中接收到的通知数据还原为权限变更通知对象。
func decodePolicyRefreshMessage(payload string) (PolicyRefreshMessage, error) {
	var message PolicyRefreshMessage
	decoder := json.NewDecoder(bytes.NewBufferString(payload))
	// 拒绝未知字段和尾随 JSON，避免生产者协议漂移被旧消费者静默接受。
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&message); err != nil {
		return PolicyRefreshMessage{}, fmt.Errorf("unmarshal rbac policy refresh message: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return PolicyRefreshMessage{}, fmt.Errorf("unmarshal rbac policy refresh message: trailing JSON data")
	}
	if err := validatePolicyRefreshMessage(message); err != nil {
		return PolicyRefreshMessage{}, err
	}
	return message, nil
}

// validatePolicyRefreshMessage 检查收到的广播通知是否合规，必须包含合法的版本号与已知的变更类型。
func validatePolicyRefreshMessage(message PolicyRefreshMessage) error {
	if message.SchemaVersion != policyRefreshSchemaVersion {
		return fmt.Errorf("unsupported rbac policy refresh schema version: %d", message.SchemaVersion)
	}
	if strings.TrimSpace(message.Reason) == "" {
		return fmt.Errorf("rbac policy refresh reason is required")
	}
	if strings.TrimSpace(message.InstanceID) == "" {
		return fmt.Errorf("rbac policy refresh publisher_instance_id is required")
	}
	for name, value := range map[string]*uuid.UUID{
		"user_id": message.UserID, "role_id": message.RoleID, "permission_id": message.PermissionID,
	} {
		if value != nil && *value == uuid.Nil() {
			return fmt.Errorf("rbac policy refresh %s must not be nil UUID", name)
		}
	}
	switch message.Kind {
	case policyRefreshKindPolicyChanged:
		if message.PolicyRevision == nil || *message.PolicyRevision <= 0 {
			return fmt.Errorf("invalid rbac policy revision")
		}
		if message.UserRoleRevision != nil {
			return fmt.Errorf("rbac policy refresh user_role_revision must be empty for %s", message.Kind)
		}
	case policyRefreshKindUserRoleChanged:
		if message.UserRoleRevision == nil || *message.UserRoleRevision <= 0 {
			return fmt.Errorf("invalid rbac user-role revision")
		}
		if message.PolicyRevision != nil {
			return fmt.Errorf("rbac policy refresh policy_revision must be empty for %s", message.Kind)
		}
		if message.UserID == nil || *message.UserID == uuid.Nil() {
			return fmt.Errorf("rbac policy refresh user_id is required for %s", message.Kind)
		}
	default:
		return fmt.Errorf("invalid rbac policy change kind: %q", message.Kind)
	}
	return nil
}

// userRoleRevision 从通知中提取用户角色变更的版本序号。
func (m PolicyRefreshMessage) userRoleRevision() int64 {
	if m.UserRoleRevision == nil {
		return 0
	}
	return *m.UserRoleRevision
}

// policyRevision 从通知中提取系统权限规则变更的版本序号。
func (m PolicyRefreshMessage) policyRevision() int64 {
	if m.PolicyRevision == nil {
		return 0
	}
	return *m.PolicyRevision
}

// policyChange 将广播消息转换成具体的权限变更事件，以便接收方决定应该清除哪些本地缓存。
func (m PolicyRefreshMessage) policyChange() permissionapplication.PolicyChange {
	kind := permissionapplication.PolicyChangeKindPolicy
	if m.Kind == policyRefreshKindUserRoleChanged {
		kind = permissionapplication.PolicyChangeKindUserRole
	}
	return permissionapplication.PolicyChange{
		Kind:         kind,
		Reason:       m.Reason,
		UserID:       uuidValue(m.UserID),
		RoleID:       uuidValue(m.RoleID),
		PermissionID: uuidValue(m.PermissionID),
	}
}

// messageRevisionLogFields 整理通知日志中的版本字段，便于在日志平台中检索对比。
func messageRevisionLogFields(message PolicyRefreshMessage) []zap.Field {
	fields := make([]zap.Field, 0, 2)
	if message.PolicyRevision != nil {
		fields = append(fields, zap.Int64("hint_policy_revision", *message.PolicyRevision))
	}
	if message.UserRoleRevision != nil {
		fields = append(fields, zap.Int64("user_role_revision", *message.UserRoleRevision))
	}
	return fields
}

// uuidValue 安全读取可能为空的唯一编号，空值按全零值处理。
func uuidValue(value *uuid.UUID) uuid.UUID {
	if value == nil {
		return uuid.Nil()
	}
	return *value
}

// optionalUUID 将全零的无效编号过滤为空，避免在广播消息中出现无意义的占位数据。
func optionalUUID(value uuid.UUID) *uuid.UUID {
	if value == uuid.Nil() {
		return nil
	}
	return &value
}

// int64Pointer 将数字转为指针，便于在传输时明确区分“未传递该字段”与“传递了数值 0”。
func int64Pointer(value int64) *int64 {
	return &value
}
