package version

import (
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Version
		wantErr bool
	}{
		{
			name:  "simple version",
			input: "1.2.3",
			want:  Version{Major: 1, Minor: 2, Patch: 3},
		},
		{
			name:  "version with v prefix",
			input: "v2.3.1",
			want:  Version{Major: 2, Minor: 3, Patch: 1},
		},
		{
			name:  "zero version",
			input: "0.0.0",
			want:  Version{Major: 0, Minor: 0, Patch: 0},
		},
		{
			name:  "large numbers",
			input: "100.200.300",
			want:  Version{Major: 100, Minor: 200, Patch: 300},
		},
		{
			name:    "invalid - too few parts",
			input:   "1.2",
			wantErr: true,
		},
		{
			name:    "invalid - too many parts",
			input:   "1.2.3.4",
			wantErr: true,
		},
		{
			name:    "invalid - non-numeric major",
			input:   "a.2.3",
			wantErr: true,
		},
		{
			name:    "invalid - non-numeric minor",
			input:   "1.b.3",
			wantErr: true,
		},
		{
			name:    "invalid - non-numeric patch",
			input:   "1.2.c",
			wantErr: true,
		},
		{
			name:    "invalid - empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("Parse(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestVersion_String(t *testing.T) {
	tests := []struct {
		version Version
		want    string
	}{
		{Version{1, 2, 3}, "1.2.3"},
		{Version{0, 0, 0}, "0.0.0"},
		{Version{10, 20, 30}, "10.20.30"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.version.String(); got != tt.want {
				t.Errorf("Version.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVersion_Bump(t *testing.T) {
	tests := []struct {
		name    string
		version Version
		level   string
		want    Version
	}{
		{
			name:    "bump major",
			version: Version{1, 2, 3},
			level:   "major",
			want:    Version{2, 0, 0},
		},
		{
			name:    "bump minor",
			version: Version{1, 2, 3},
			level:   "minor",
			want:    Version{1, 3, 0},
		},
		{
			name:    "bump patch",
			version: Version{1, 2, 3},
			level:   "patch",
			want:    Version{1, 2, 4},
		},
		{
			name:    "bump none",
			version: Version{1, 2, 3},
			level:   "none",
			want:    Version{1, 2, 3},
		},
		{
			name:    "bump unknown",
			version: Version{1, 2, 3},
			level:   "unknown",
			want:    Version{1, 2, 3},
		},
		{
			name:    "bump major from zero",
			version: Version{0, 0, 0},
			level:   "major",
			want:    Version{1, 0, 0},
		},
		{
			name:    "bump minor from zero",
			version: Version{0, 0, 0},
			level:   "minor",
			want:    Version{0, 1, 0},
		},
		{
			name:    "bump patch from zero",
			version: Version{0, 0, 0},
			level:   "patch",
			want:    Version{0, 0, 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.version.Bump(tt.level); got != tt.want {
				t.Errorf("Version.Bump(%q) = %v, want %v", tt.level, got, tt.want)
			}
		})
	}
}

func TestZero(t *testing.T) {
	z := Zero()
	if z.Major != 0 || z.Minor != 0 || z.Patch != 0 {
		t.Errorf("Zero() = %v, want 0.0.0", z)
	}
}
