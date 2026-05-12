package tui

import "testing"

func TestDeduplicateBarcode(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "barcode duplicado exacto",
			input: "12341234",
			want:  "1234",
		},
		{
			name:  "barcode duplicado con longitud impar de base",
			input: "ABCABC",
			want:  "ABC",
		},
		{
			name:  "longitud par pero mitades distintas",
			input: "12345678",
			want:  "12345678",
		},
		{
			name:  "longitud impar — nunca duplicado",
			input: "12345",
			want:  "12345",
		},
		{
			name:  "string vacío",
			input: "",
			want:  "",
		},
		{
			name:  "un solo carácter",
			input: "X",
			want:  "X",
		},
		{
			name:  "dos caracteres iguales — duplicado de un char",
			input: "AA",
			want:  "A",
		},
		{
			name:  "barcode EAN-13 normal sin duplicar",
			input: "7891234567890",
			want:  "7891234567890",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deduplicateBarcode(tt.input)
			if got != tt.want {
				t.Errorf("deduplicateBarcode(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
