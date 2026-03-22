package graph

import (
	"encoding/base64"
	"testing"
)

func TestDecodeUID(t *testing.T) {
	tests := []struct {
		name    string
		uid     string
		want    int
		wantErr bool
	}{
		{"valid id 1", base64.StdEncoding.EncodeToString([]byte("1")), 1, false},
		{"valid id 42", base64.StdEncoding.EncodeToString([]byte("42")), 42, false},
		{"valid id 999", base64.StdEncoding.EncodeToString([]byte("999")), 999, false},
		{"invalid base64", "not-valid-base64!!!", 0, true},
		{"valid base64 non-numeric", base64.StdEncoding.EncodeToString([]byte("abc")), 0, true},
		{"empty", "", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodeUID(tt.uid)
			if (err != nil) != tt.wantErr {
				t.Errorf("decodeUID(%q) error = %v, wantErr %v", tt.uid, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("decodeUID(%q) = %d, want %d", tt.uid, got, tt.want)
			}
		})
	}
}

func TestDerefStr(t *testing.T) {
	s := "hello"
	if derefStr(&s) != "hello" {
		t.Error("derefStr should return pointed-to value")
	}
	if derefStr(nil) != "" {
		t.Error("derefStr(nil) should return empty string")
	}
}

func TestCoalesce(t *testing.T) {
	a := "first"
	b := "second"

	if result := coalesce(&a, &b); result == nil || *result != "first" {
		t.Error("coalesce should return first non-nil value")
	}
	if result := coalesce(nil, &b); result == nil || *result != "second" {
		t.Error("coalesce should skip nil and return second")
	}
	if result := coalesce(nil, nil); result != nil {
		t.Error("coalesce of all nils should return nil")
	}
}
