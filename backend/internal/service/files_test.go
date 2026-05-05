package service

import (
	"testing"
	"time"
)

func TestTaskImageDirUsesYearMonthDay(t *testing.T) {
	createdAt := time.Date(2026, time.May, 3, 14, 0, 37, 0, time.UTC)

	got := TaskImageDir(createdAt, "task_abc123")
	want := "images/2026/05/03/task_abc123"
	if got != want {
		t.Fatalf("TaskImageDir() = %q, want %q", got, want)
	}
}
