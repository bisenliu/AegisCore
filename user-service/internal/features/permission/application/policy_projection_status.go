package application

// PolicyProjectionStatus 是 Casbin policy 投影的一致性只读快照。
type PolicyProjectionStatus struct {
	Initialized     bool
	ReloadSucceeded bool
	AppliedRevision int64
	TargetRevision  int64
	LastError       error
}

// Ready 返回当前投影是否已成功追平 engine 观察到的最高目标 revision。
func (s PolicyProjectionStatus) Ready() bool {
	return s.Initialized && s.ReloadSucceeded && s.LastError == nil && s.AppliedRevision >= s.TargetRevision
}
