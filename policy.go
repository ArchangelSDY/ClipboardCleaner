package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const clipboardTimeout = 10 * time.Second
const maxTimeoutSeconds = int64(1<<63-1) / int64(time.Second)

func parseTimeout(text string) (time.Duration, error) {
	text = strings.TrimSpace(text)
	seconds, err := strconv.ParseUint(text, 10, 64)
	if err != nil || seconds == 0 || seconds > uint64(maxTimeoutSeconds) {
		return 0, fmt.Errorf("Enter a whole number from 1 to %d seconds.", maxTimeoutSeconds)
	}
	return time.Duration(seconds) * time.Second, nil
}

type clipboardPolicy struct {
	sequence uint32
	deadline time.Time
	armed    bool
	enabled  bool
	timeout  time.Duration
}

func newClipboardPolicy(sequence uint32, now time.Time) clipboardPolicy {
	return clipboardPolicy{sequence: sequence, deadline: now.Add(clipboardTimeout), armed: true, enabled: true, timeout: clipboardTimeout}
}

func (p *clipboardPolicy) start(sequence uint32, now time.Time) {
	p.sequence = sequence
	p.deadline = now.Add(p.timeout)
	p.armed = true
	p.enabled = true
}

func (p *clipboardPolicy) setTimeout(timeout time.Duration, sequence uint32, now time.Time) {
	p.timeout = timeout
	if p.enabled {
		p.start(sequence, now)
	}
}

func (p *clipboardPolicy) stop() {
	p.enabled = false
	p.armed = false
}

func (p *clipboardPolicy) observe(sequence uint32, now time.Time) {
	if !p.enabled {
		return
	}
	if sequence != p.sequence {
		p.sequence = sequence
		p.deadline = now.Add(p.timeout)
		p.armed = true
	}
}

func (p *clipboardPolicy) due(now time.Time) bool {
	return p.enabled && p.armed && !now.Before(p.deadline)
}

func (p *clipboardPolicy) mayClear(sequence uint32, now time.Time) bool {
	p.observe(sequence, now)
	return p.due(now)
}

func (p *clipboardPolicy) cleared(sequence uint32) {
	p.sequence = sequence
	p.armed = false
}
