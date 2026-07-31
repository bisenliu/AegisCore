package redis

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/google/uuid"

	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
)

const policyRefreshSchemaVersion = 1

const (
	policyRefreshKindPolicyChanged   = "policy_changed"
	policyRefreshKindUserRoleChanged = "user_role_changed"
)

// PolicyRefreshMessage 是版本化的 RBAC policy Redis Pub/Sub 刷新 envelope。
type PolicyRefreshMessage struct {
	SchemaVersion  int        `json:"schema_version"`
	EventID        uuid.UUID  `json:"event_id"`
	IdempotencyKey string     `json:"idempotency_key"`
	PolicyRevision int64      `json:"policy_revision"`
	Kind           string     `json:"kind"`
	Reason         string     `json:"reason"`
	InstanceID     string     `json:"publisher_instance_id"`
	UserID         *uuid.UUID `json:"user_id,omitempty"`
	RoleID         *uuid.UUID `json:"role_id,omitempty"`
	PermissionID   *uuid.UUID `json:"permission_id,omitempty"`
}

func newPolicyRefreshMessage(event permissionapplication.OutboxEvent, instanceID string) PolicyRefreshMessage {
	return PolicyRefreshMessage{
		SchemaVersion:  policyRefreshSchemaVersion,
		EventID:        event.EventID,
		IdempotencyKey: event.IdempotencyKey,
		PolicyRevision: event.Revision,
		Kind:           event.Kind,
		Reason:         event.Reason,
		InstanceID:     instanceID,
		UserID:         event.UserID,
		RoleID:         event.RoleID,
		PermissionID:   event.PermissionID,
	}
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
	if message.EventID == uuid.Nil {
		return fmt.Errorf("rbac policy refresh event_id is required")
	}
	if strings.TrimSpace(message.IdempotencyKey) == "" {
		return fmt.Errorf("rbac policy refresh idempotency_key is required")
	}
	if message.PolicyRevision <= 0 {
		return fmt.Errorf("invalid rbac policy revision: %d", message.PolicyRevision)
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
		if value != nil && *value == uuid.Nil {
			return fmt.Errorf("rbac policy refresh %s must not be nil UUID", name)
		}
	}
	switch message.Kind {
	case policyRefreshKindPolicyChanged:
	case policyRefreshKindUserRoleChanged:
		if message.UserID == nil || *message.UserID == uuid.Nil {
			return fmt.Errorf("rbac policy refresh user_id is required for %s", message.Kind)
		}
	default:
		return fmt.Errorf("invalid rbac policy change kind: %q", message.Kind)
	}
	return nil
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

func uuidValue(value *uuid.UUID) uuid.UUID {
	if value == nil {
		return uuid.Nil
	}
	return *value
}
