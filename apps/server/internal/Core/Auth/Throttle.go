package auth

import "time"

const maxShift = 30

func LockDuration(failures, free int, base, maxLock time.Duration) time.Duration {
	if failures < free || free < 1 || base <= 0 {
		return 0
	}
	shift := min(failures-free, maxShift)
	d := base << shift
	if d <= 0 || d > maxLock {
		return maxLock
	}
	return d
}

func AccountThrottleKey(emailHash string) string { return "account:" + emailHash }

func IPThrottleKey(ip string) string { return "ip:" + ip }
