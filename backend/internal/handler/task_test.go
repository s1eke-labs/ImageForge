package handler

import "testing"

func TestThumbPathUsesTaskDirectory(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "year month day directory",
			path: "images/2026/05/03/task_abc123/output/result.png",
			want: "images/2026/05/03/task_abc123/thumbs/result.jpg",
		},
		{
			name: "legacy year month directory",
			path: "images/2026-05/task_abc123/output/result.png",
			want: "images/2026-05/task_abc123/thumbs/result.jpg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := thumbPath(tt.path, "result.jpg")
			if got != tt.want {
				t.Fatalf("thumbPath() = %q, want %q", got, tt.want)
			}
		})
	}
}
