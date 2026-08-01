package outbox

import "time"

func NextRetryAvailableAt(attempt int, now time.Time) time.Time {
	switch attempt {
	case 1:
		return now.Add(5 * time.Second)
	case 2:
		return now.Add(15 * time.Second)
	case 3:
		return now.Add(60 * time.Second)
	case 4:
		return now.Add(5 * time.Minute)
	default:
		return now.Add(5 * time.Minute)
	}
}
