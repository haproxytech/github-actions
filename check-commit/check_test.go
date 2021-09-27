package main

import (
	"testing"
)

func TestCheckSubject(t *testing.T) {
	t.Parallel()

	c, _ := LoadCommitPolicy("")

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := c.CheckSubject([]byte(tt.subject)); (err != nil) != tt.wantErr {
				t.Errorf("checkSubject() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
