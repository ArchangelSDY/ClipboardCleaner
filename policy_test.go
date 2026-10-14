package main

import (
	"testing"
	"time"
)

func TestStartupGrace(t *testing.T) {
	start := time.Now()
	p := newClipboardPolicy(10, start)
	if p.due(start.Add(clipboardTimeout - time.Nanosecond)) {
		t.Fatal("clipboard expired before five seconds")
	}
	if !p.due(start.Add(clipboardTimeout)) {
		t.Fatal("clipboard did not expire at five seconds")
	}
}

func TestNewCopyResetsDeadline(t *testing.T) {
	start := time.Now()
	p := newClipboardPolicy(10, start)
	p.observe(11, start.Add(4*time.Second))
	p.observe(11, start.Add(8*time.Second))
	if p.due(start.Add(8 * time.Second)) {
		t.Fatal("new copy did not receive a full grace period")
	}
	if !p.due(start.Add(9 * time.Second)) {
		t.Fatal("unchanged sequence incorrectly extended the deadline")
	}
}

func TestCopyWhileAcquiringLockIsProtected(t *testing.T) {
	start := time.Now()
	p := newClipboardPolicy(10, start)
	if p.mayClear(11, start.Add(6*time.Second)) {
		t.Fatal("fresh copy discovered under lock would be deleted")
	}
	if !p.mayClear(11, start.Add(11*time.Second)) {
		t.Fatal("fresh copy never expired")
	}
}

func TestOwnClearDoesNotRearm(t *testing.T) {
	start := time.Now()
	p := newClipboardPolicy(10, start)
	p.cleared(11)
	p.observe(11, start.Add(6*time.Second))
	if p.due(start.Add(time.Hour)) {
		t.Fatal("own clear rearmed expiration")
	}
	p.observe(12, start.Add(time.Hour))
	if p.due(start.Add(time.Hour + clipboardTimeout - time.Nanosecond)) {
		t.Fatal("copy after clear expired early")
	}
	if !p.due(start.Add(time.Hour + clipboardTimeout)) {
		t.Fatal("copy after clear did not expire")
	}
}

func TestSequenceWraparound(t *testing.T) {
	start := time.Now()
	p := newClipboardPolicy(^uint32(0), start)
	if p.mayClear(0, start.Add(clipboardTimeout)) {
		t.Fatal("wrapped sequence was not treated as a fresh copy")
	}
}

func TestContentionDoesNotExtendDeadline(t *testing.T) {
	start := time.Now()
	p := newClipboardPolicy(10, start)
	for _, elapsed := range []time.Duration{5, 6, 7} {
		if !p.mayClear(10, start.Add(elapsed*time.Second)) {
			t.Fatal("retry of unchanged clipboard extended deadline")
		}
	}
}
