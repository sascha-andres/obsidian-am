package main

import "testing"

func TestIsZeroValue(t *testing.T) {
	tests := []struct {
		name       string
		value      any
		present    bool
		wantIsZero bool
		wantOk     bool
	}{
		{"missing key", nil, false, true, true},
		{"int zero", 0, true, true, true},
		{"int non-zero", 100, true, false, true},
		{"int64 zero", int64(0), true, true, true},
		{"float zero", 0.0, true, true, true},
		{"float non-zero", 82.5, true, false, true},
		{"string zero", "0", true, true, true},
		{"string non-zero", "100", true, false, true},
		{"string with whitespace", " 0 ", true, true, true},
		{"garbage string", "n/a", true, false, false},
		{"unsupported type", true, true, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotIsZero, gotOk := isZeroValue(tt.value, tt.present)
			if gotOk != tt.wantOk {
				t.Fatalf("isZeroValue(%v, %v) ok = %v, want %v", tt.value, tt.present, gotOk, tt.wantOk)
			}
			if gotOk && gotIsZero != tt.wantIsZero {
				t.Fatalf("isZeroValue(%v, %v) isZero = %v, want %v", tt.value, tt.present, gotIsZero, tt.wantIsZero)
			}
		})
	}
}
