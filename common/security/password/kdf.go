package password

import (
	"context"

	"golang.org/x/crypto/argon2"
)

func (s *Service) deriveKeyContext(ctx context.Context, plain, salt []byte, params passwordParams) ([]byte, error) {
	// queue 限制等待中的 KDF 请求，gate 限制实际 Argon2 并发；两层都响应 ctx 取消，用于在过载时快速背压。
	if err := s.enterArgon2Queue(ctx); err != nil {
		return nil, err
	}
	defer s.leaveArgon2Queue()

	if err := s.acquireArgon2Slot(ctx); err != nil {
		return nil, err
	}
	defer s.releaseArgon2Slot()

	return argon2.IDKey(
		plain,
		salt,
		params.iterations,
		params.memory,
		params.parallelism,
		params.keyLength,
	), nil
}

func (s *Service) enterArgon2Queue(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	select {
	case s.queue <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	default:
		// 队列满时立即拒绝，避免密码哈希请求无限排队拖垮进程。
		return ErrPasswordKDFBusy
	}

	if err := ctx.Err(); err != nil {
		s.leaveArgon2Queue()
		return err
	}

	return nil
}

func (s *Service) leaveArgon2Queue() {
	<-s.queue
}

func (s *Service) acquireArgon2Slot(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	select {
	case s.gate <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}

	if err := ctx.Err(); err != nil {
		s.releaseArgon2Slot()
		return err
	}

	return nil
}

func (s *Service) releaseArgon2Slot() {
	<-s.gate
}
