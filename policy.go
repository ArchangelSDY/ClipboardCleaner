package main

import "time"

const clipboardTimeout = 5 * time.Second

type clipboardPolicy struct {
	sequence uint32
	deadline time.Time
	armed    bool
}

func newClipboardPolicy(sequence uint32, now time.Time) clipboardPolicy {
	return clipboardPolicy{sequence: sequence, deadline: now.Add(clipboardTimeout), armed: true}
}

func (p *clipboardPolicy) observe(sequence uint32, now time.Time) {
	if sequence != p.sequence {
		p.sequence = sequence
		p.deadline = now.Add(clipboardTimeout)
		p.armed = true
	}
}

func (p *clipboardPolicy) due(now time.Time) bool {
	return p.armed && !now.Before(p.deadline)
}

func (p *clipboardPolicy) mayClear(sequence uint32, now time.Time) bool {
	p.observe(sequence, now)
	return p.due(now)
}

func (p *clipboardPolicy) cleared(sequence uint32) {
	p.sequence = sequence
	p.armed = false
}
