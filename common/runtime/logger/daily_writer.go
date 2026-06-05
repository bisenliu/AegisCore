package logger

import (
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/aegiscore/common/runtime/config"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

type dailyLumberjackWriteSyncer struct {
	mu        sync.Mutex
	dir       string
	prefix    string
	levelName string
	date      string
	newClock  func() time.Time
	logger    *lumberjack.Logger
	cfg       config.LogConfig
}

func newDailyLumberjackWriteSyncer(filename string, cfg config.LogConfig) zapcore.WriteSyncer {
	prefix, levelName := splitLogFilename(filepath.Base(filename))
	w := &dailyLumberjackWriteSyncer{
		dir:       filepath.Dir(filename),
		prefix:    prefix,
		levelName: levelName,
		newClock:  time.Now,
		cfg:       cfg,
	}
	_ = w.rotateLocked()
	return zapcore.AddSync(w)
}

func (w *dailyLumberjackWriteSyncer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if currentDate := w.newClock().Format("2006-01-02"); currentDate != w.date {
		if err := w.rotateLocked(); err != nil {
			return 0, err
		}
	}
	return w.logger.Write(p)
}

func (w *dailyLumberjackWriteSyncer) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.logger == nil {
		return nil
	}
	return nil
}

func (w *dailyLumberjackWriteSyncer) rotateLocked() error {
	date := w.newClock().Format("2006-01-02")
	if w.logger != nil {
		if err := w.logger.Close(); err != nil {
			return err
		}
	}
	w.date = date
	w.logger = &lumberjack.Logger{
		Filename:   w.datedFilename(date),
		MaxSize:    positiveOrDefault(w.cfg.MaxSizeMB, 100),
		MaxBackups: positiveOrDefault(w.cfg.MaxBackups, 30),
		MaxAge:     positiveOrDefault(w.cfg.MaxAgeDays, 7),
		LocalTime:  true,
		Compress:   false,
	}
	return nil
}

func (w *dailyLumberjackWriteSyncer) datedFilename(date string) string {
	return filepath.Join(w.dir, fmt.Sprintf("%s.%s.%s.log", w.prefix, date, w.levelName))
}
