package main

import "testing"

func TestCheckSubject(t *testing.T) {
	t.Parallel()

	c, _ := LoadCommitPolicy("")

	type args struct {
		subject string
	}

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "valid type and severity",
			args:    args{subject: "BUG/MEDIUM: config: add default location of path to the configuration file"},
			wantErr: false,
		},
		{
			name:    "short subject",
			args:    args{subject: "BUG/MEDIUM: config: default"},
			wantErr: true,
		},
		{
			name:    "missing severity",
			args:    args{subject: "BUG/: config: default implementation"},
			wantErr: true,
		},
		{
			name:    "wrong tag",
			args:    args{subject: "WRONG: config: default implementation"},
			wantErr: true,
		},
		{
			name:    "wrong severity",
			args:    args{subject: "BUG/WRONG: config: default implementation"},
			wantErr: true,
		},
		{
			name:    "double spaces",
			args:    args{subject: "BUG/MEDIUM: config:  default implementation"},
			wantErr: true,
		},
		{
			name:    "trailing spaces",
			args:    args{subject: "BUG/MEDIUM: config: default implementation "},
			wantErr: true,
		},
		{
			name:    "unprocessed tags remain",
			args:    args{subject: "BUG/MINOR: MAJOR: config: default implementation"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := c.CheckSubject([]byte(tt.args.subject)); (err != nil) != tt.wantErr {
				t.Errorf("checkSubject() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
