package probe

import "testing"

func TestErrorMessage(t *testing.T) {
	path := `C:\Media\Broken (2010).mkv`
	tests := []struct{ stderr, want string }{
		{"[matroska,webm @ 0x1a2b3c] Truncating packet of size 10611 to 21\r\n" + path + ": Invalid data found when processing input\r\n",
			"Invalid data found when processing input"},
		{"[mov,mp4,m4a,3gp,3g2,mj2 @ 0x55d0] moov atom not found\n", "moov atom not found"},
		{"  \n", ""},
	}
	for _, tt := range tests {
		if got := errorMessage(tt.stderr, path); got != tt.want {
			t.Errorf("errorMessage(%q) = %q, want %q", tt.stderr, got, tt.want)
		}
	}
}
