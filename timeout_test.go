package main

import (
	"strconv"
	"testing"
	"time"
)

func TestDefaultIsTenSeconds(t *testing.T) {
	p := newClipboardPolicy(1, time.Now())
	if p.timeout != 10*time.Second {
		t.Fatal("startup default must be ten seconds")
	}
}

func TestParseTimeout(t *testing.T) {
	for _, text := range []string{"", "0", "-1", "+1", "1.5", "abc", "1s", "1 2",
		strconv.FormatInt(maxTimeoutSeconds+1, 10), "18446744073709551616"} {
		if _, err := parseTimeout(text); err == nil {
			t.Errorf("accepted invalid timeout %q", text)
		}
	}
	for _, tc := range []struct {
		text string
		want time.Duration
	}{
		{"1", time.Second},
		{" 27 ", 27 * time.Second},
		{strconv.FormatInt(maxTimeoutSeconds, 10), time.Duration(maxTimeoutSeconds) * time.Second},
	} {
		got, err := parseTimeout(tc.text)
		if err != nil || got != tc.want {
			t.Errorf("parseTimeout(%q) = %v, %v; want %v", tc.text, got, err, tc.want)
		}
	}
}

func TestTimeoutChangeRestartsCountdown(t *testing.T) {
	for _, timeout := range []time.Duration{time.Second, 30 * time.Second} {
		start := time.Now()
		p := newClipboardPolicy(1, start)
		change := start.Add(time.Hour)
		p.setTimeout(timeout, 2, change)
		if p.sequence != 2 || p.due(change.Add(timeout-time.Nanosecond)) || !p.due(change.Add(timeout)) {
			t.Fatal("timeout change did not reset countdown with current sequence")
		}
		if p.mayClear(3, change.Add(timeout)) {
			t.Fatal("timeout change lost fresh-copy protection")
		}
		if !p.due(change.Add(2 * timeout)) {
			t.Fatal("new copy did not use configured timeout")
		}
	}
}

func TestTimeoutWhileStoppedStaysStoppedAndSurvivesStart(t *testing.T) {
	start := time.Now()
	p := newClipboardPolicy(1, start)
	p.stop()
	p.setTimeout(27*time.Second, 2, start)
	if p.enabled || p.due(start.Add(time.Hour)) {
		t.Fatal("changing timeout started a stopped cleaner")
	}
	p.start(3, start)
	if p.timeout != 27*time.Second || p.due(start.Add(27*time.Second-time.Nanosecond)) || !p.due(start.Add(27*time.Second)) {
		t.Fatal("start did not preserve timeout")
	}
	if fresh := newClipboardPolicy(4, start); fresh.timeout != 10*time.Second {
		t.Fatal("configured timeout leaked to new instance")
	}
}

func TestTimeoutChangeAfterClearRearmsGrace(t *testing.T) {
	now := time.Now()
	p := newClipboardPolicy(1, now)
	p.cleared(2)
	p.setTimeout(20*time.Second, 2, now)
	if p.due(now) || !p.due(now.Add(20*time.Second)) {
		t.Fatal("applying timeout did not give existing clipboard full grace")
	}
}
