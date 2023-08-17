package logclient

import (
	"testing"
)

func TestNewPageAligned(t *testing.T) {
	page := NewPage(0, 32)
	if page.start != 0 && page.end != 31 {
		t.Fail()
	}
}

func TestNewPageUnaligned(t *testing.T) {
	var page Page
	page = NewPage(5, 32)
	if page.start != 5 && page.end != 31 {
		t.Fail()
	}
	page = NewPage(35, 32)
	if page.start != 35 && page.end != 61 {
		t.Fail()
	}
}
