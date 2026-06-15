package logger

import "go.uber.org/zap/zapcore"

type namedLoggerCore struct {
	zapcore.Core
	name string
}

type excludedLoggerCore struct {
	zapcore.Core
	name string
}

func newNamedLoggerCore(core zapcore.Core, name string) zapcore.Core {
	return namedLoggerCore{Core: core, name: name}
}

func newExcludedLoggerCore(core zapcore.Core, name string) zapcore.Core {
	return excludedLoggerCore{Core: core, name: name}
}

func (c namedLoggerCore) With(fields []zapcore.Field) zapcore.Core {
	return namedLoggerCore{Core: c.Core.With(fields), name: c.name}
}

func (c namedLoggerCore) Check(entry zapcore.Entry, checked *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if entry.LoggerName != c.name || !c.Enabled(entry.Level) {
		return checked
	}
	return checked.AddCore(entry, c)
}

func (c excludedLoggerCore) With(fields []zapcore.Field) zapcore.Core {
	return excludedLoggerCore{Core: c.Core.With(fields), name: c.name}
}

func (c excludedLoggerCore) Check(entry zapcore.Entry, checked *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if entry.LoggerName == c.name || !c.Enabled(entry.Level) {
		return checked
	}
	return checked.AddCore(entry, c)
}
