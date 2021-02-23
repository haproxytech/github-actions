package main

import "testing"

func Test_checkSubject(t *testing.T) {
	type args struct {
		subject string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "bug",
			args:    args{subject: "BUG/MEDIUM: config: add default location of path to the configuration file"},
			wantErr: false,
		},
		{
			name:    "bug-fail",
			args:    args{subject: "BUG/MEDIUM: config: default"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := checkSubject(tt.args.subject); (err != nil) != tt.wantErr {
				t.Errorf("checkSubject() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
