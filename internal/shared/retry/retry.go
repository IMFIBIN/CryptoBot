package retry

import "time"

// WithRetry выполняет op с повторами и простым экспоненциальным бэкоффом.
func WithRetry(attempts int, sleep time.Duration, op func() error) error {
	var err error
	backoff := sleep
	for i := 0; i < attempts; i++ {
		if err = op(); err == nil {
			return nil
		}
		time.Sleep(backoff)
		if backoff < 5*time.Second {
			backoff *= 2
		}
	}
	return err
}
