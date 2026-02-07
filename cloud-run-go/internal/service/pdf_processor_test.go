package service

import (
	"reflect"
	"testing"
)

func TestSortPdftoppmJPGs_NumericOrder(t *testing.T) {
	names := []string{
		"page-10.jpg",
		"page-2.jpg",
		"page-1.jpg",
	}

	sortPdftoppmJPGs(names)

	want := []string{"page-1.jpg", "page-2.jpg", "page-10.jpg"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("unexpected sort: got=%v want=%v", names, want)
	}
}

func TestParsePdftoppmPageNumber(t *testing.T) {
	tests := []struct {
		name string
		want int
		ok   bool
	}{
		{"page-1.jpg", 1, true},
		{"page-10.jpg", 10, true},
		{"page-a.jpg", 0, false},
		{"page.jpg", 0, false},
		{"page-0.jpg", 0, false},
		{"page-1.png", 0, false},
	}

	for _, tt := range tests {
		got, ok := parsePdftoppmPageNumber(tt.name)
		if got != tt.want || ok != tt.ok {
			t.Fatalf("parsePdftoppmPageNumber(%q) = (%d,%v), want (%d,%v)", tt.name, got, ok, tt.want, tt.ok)
		}
	}
}

