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

func (m PolicyRefreshMessage) userRoleRevision() int64 {
	if m.UserRoleRevision == nil {
		return 0
	}
	return *m.UserRoleRevision
}

func (m PolicyRefreshMessage) policyRevision() int64 {
	if m.PolicyRevision == nil {
		return 0
	}
	return *m.PolicyRevision
}

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

func uuidValue(value *uuid.UUID) uuid.UUID {
	if value == nil {
		return uuid.Nil()
	}
	return *value
}

func optionalUUID(value uuid.UUID) *uuid.UUID {
	if value == uuid.Nil() {
		return nil
	}
	return &value
}

func int64Pointer(value int64) *int64 {
	return &value
}
