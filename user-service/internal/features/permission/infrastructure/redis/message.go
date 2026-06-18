package redis

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
)

// PolicyRefreshMessage 是 RBAC policy Redis Pub/Sub 刷新消息。
type PolicyRefreshMessage struct {
	Version      int64                                  `json:"version"`
	InstanceID   string                                 `json:"instance_id"`
	Kind         permissionapplication.PolicyChangeKind `json:"kind"`
	Reason       string                                 `json:"reason"`
	UserID       uuid.UUID                              `json:"user_id"`
	RoleID       uuid.UUID                              `json:"role_id"`
	PermissionID uuid.UUID                              `json:"permission_id"`
	PublishedAt  int64                                  `json:"published_at"`
}

func newPolicyRefreshMessage(version int64, instanceID string, change permissionapplication.PolicyChange) PolicyRefreshMessage {
	return PolicyRefreshMessage{
		Version:      version,
		InstanceID:   instanceID,
		Kind:         change.Kind,
		Reason:       change.ReasonText(),
		UserID:       change.UserID,
		RoleID:       change.RoleID,
		PermissionID: change.PermissionID,
		PublishedAt:  time.Now().Unix(),
	}
}

func encodePolicyRefreshMessage(message PolicyRefreshMessage) (string, error) {
	payload, err := json.Marshal(message)
	if err != nil {
		return "", fmt.Errorf("marshal rbac policy refresh message: %w", err)
	}
	return string(payload), nil
}

func decodePolicyRefreshMessage(payload string) (PolicyRefreshMessage, error) {
	var message PolicyRefreshMessage
	if err := json.Unmarshal([]byte(payload), &message); err != nil {
		return PolicyRefreshMessage{}, fmt.Errorf("unmarshal rbac policy refresh message: %w", err)
	}
	if message.Version <= 0 {
		return PolicyRefreshMessage{}, fmt.Errorf("invalid rbac policy version: %d", message.Version)
	}
	switch message.Kind {
	case permissionapplication.PolicyChangeKindPolicy, permissionapplication.PolicyChangeKindUserRole:
	default:
		return PolicyRefreshMessage{}, fmt.Errorf("invalid rbac policy change kind: %q", message.Kind)
	}
	return message, nil
}

func (m PolicyRefreshMessage) policyChange() permissionapplication.PolicyChange {
	return permissionapplication.PolicyChange{
		Kind:         m.Kind,
		Reason:       m.Reason,
		UserID:       m.UserID,
		RoleID:       m.RoleID,
		PermissionID: m.PermissionID,
	}
}
