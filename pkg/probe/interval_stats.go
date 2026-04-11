package probe

import "time"

type intervalStats struct {
	lastAt    time.Time
	lastBytes int64
	seen      bool
	peakMbps  float64
}

func (s *intervalStats) Observe(now time.Time, totalBytes int64) {
	if s == nil || totalBytes < 0 {
		return
	}
	if now.IsZero() {
		now = time.Now()
	}
	if !s.seen {
		s.seen = true
		s.lastAt = now
		s.lastBytes = totalBytes
		return
	}
	if totalBytes < s.lastBytes {
		return
	}
	if totalBytes == s.lastBytes {
		return
	}
	elapsed := now.Sub(s.lastAt)
	if elapsed <= 0 {
		return
	}
	bytesDelta := totalBytes - s.lastBytes
	mbps := float64(bytesDelta*8) / elapsed.Seconds() / 1_000_000
	if mbps > s.peakMbps {
		s.peakMbps = mbps
	}
	s.lastAt = now
	s.lastBytes = totalBytes
}

func (s *intervalStats) PeakMbps() float64 {
	if s == nil {
		return 0
	}
	return s.peakMbps
}
