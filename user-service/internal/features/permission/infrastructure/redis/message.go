package redis

import (
	"encoding/json"
	"fmt"
	"time"
)

// PolicyRefreshMessage 是 RBAC policy Redis Pub/Sub 刷新消息。
type PolicyRefreshMessage struct {
	Version     int64  `json:"version"`
	InstanceID  string `json:"instance_id"`
	Reason      string `json:"reason"`
	PublishedAt int64  `json:"published_at"`
}

func newPolicyRefreshMessage(version int64, instanceID string, reason string) PolicyRefreshMessage {
	return PolicyRefreshMessage{Version: version, InstanceID: instanceID, Reason: reason, PublishedAt: time.Now().Unix()}
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
	return message, nil
}
