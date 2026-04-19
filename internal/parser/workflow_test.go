package parser

import (
	"reflect"
	"testing"
)

func TestSplitWorkflow(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "then and after that",
			input: "find old logs then compress them after that move to archive",
			want:  []string{"find old logs", "compress them", "move to archive"},
		},
		{
			name:  "and then",
			input: "list files and then copy to usb",
			want:  []string{"list files", "copy to usb"},
		},
		{
			name:  "comma fallback",
			input: "find logs, compress them, move to archive",
			want:  []string{"find logs", "compress them", "move to archive"},
		},
		{
			name:  "single step",
			input: "show disk usage",
			want:  []string{"show disk usage"},
		},
		{
			name:  "empty",
			input: "   ",
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitWorkflow(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("SplitWorkflow(%q) = %#v, want %#v", tt.input, got, tt.want)
			}
		})
	}
}
