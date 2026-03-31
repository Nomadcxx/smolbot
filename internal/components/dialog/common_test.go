package dialog

import "testing"

func TestDialogWidth(t *testing.T) {
	tests := []struct {
		termWidth int
		preferred int
		want      int
	}{
		{termWidth: 0, preferred: 64, want: 64},
		{termWidth: 100, preferred: 64, want: 64},
		{termWidth: 60, preferred: 64, want: 56},
		{termWidth: 40, preferred: 72, want: 36},
		{termWidth: 64, preferred: 64, want: 60},
	}
	for _, tt := range tests {
		got := dialogWidth(tt.termWidth, tt.preferred)
		if got != tt.want {
			t.Errorf("dialogWidth(%d, %d) = %d, want %d", tt.termWidth, tt.preferred, got, tt.want)
		}
	}
}
