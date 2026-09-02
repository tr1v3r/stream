package tests

import (
	"reflect"
	"testing"
)

// Table-driven tests for the exercise answers that had no assertions at all
// (previously only executed via t.Logf smoke runs).

func TestQuestion1Sub4(t *testing.T) {
	tests := []struct {
		name string
		in   []*Employee
		want map[int][]int64
	}{
		{
			name: "empty input yields empty map",
			in:   nil,
			want: map[int][]int64{},
		},
		{
			name: "groups ids by age preserving input order",
			in: []*Employee{
				{ID: 1, Age: 25},
				{ID: 2, Age: 30},
				{ID: 3, Age: 25},
				{ID: 4, Age: 30},
				{ID: 5, Age: 25},
			},
			want: map[int][]int64{
				25: {1, 3, 5},
				30: {2, 4},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Question1Sub4(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Question1Sub4() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestQuestion2Sub1(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want int64
	}{
		{name: "empty string", in: "", want: 0},
		{name: "all lowercase", in: "abcxyz", want: 6},
		{name: "mixed case counts only lowercase", in: "Hello World", want: 8},
		{name: "digits and punctuation are not lowercase letters", in: "A1B2!@", want: 0},
		{name: "byte range boundary: ` (96) and { (123) excluded", in: "`a{", want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Question2Sub1(tt.in); got != tt.want {
				t.Errorf("Question2Sub1(%q) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestQuestion2Sub2(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want string
	}{
		{name: "empty list", in: nil, want: ""},
		{name: "single element", in: []string{"abc"}, want: "abc"},
		{name: "picks string with most lowercase letters", in: []string{"ABC", "aBCdE", "abcdef"}, want: "abcdef"},
		{name: "tie keeps the first maximal string", in: []string{"xyz", "abc", "qqq"}, want: "xyz"},
		{name: "all-uppercase list never beats max=0, stays empty", in: []string{"A", "B"}, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Question2Sub2(tt.in); got != tt.want {
				t.Errorf("Question2Sub2(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
