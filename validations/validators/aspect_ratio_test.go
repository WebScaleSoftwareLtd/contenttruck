package validators

import (
	"strconv"
	"testing"
)

func Test_calculateAspectRatio(t *testing.T) {
	tests := []struct {
		width  int
		height int
		ratio  string
	}{
		{0, 0, "0:0"},
		{1, 1, "1:1"},
		{1920, 1080, "16:9"},
		{1280, 720, "16:9"},
		{640, 480, "4:3"},
		{320, 240, "4:3"},
		{160, 120, "4:3"},
		{800, 600, "4:3"},
		{400, 300, "4:3"},
	}
	for _, tt := range tests {
		t.Run(strconv.Itoa(tt.width)+":"+strconv.Itoa(tt.height), func(t *testing.T) {
			if got := calculateAspectRatio(tt.width, tt.height); got != tt.ratio {
				t.Errorf("calculateAspectRatio() = %v, want %v", got, tt.ratio)
			}
		})
	}
}
