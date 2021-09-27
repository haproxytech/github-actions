package main

var tests = []struct {
	name    string
	subject string
	wantErr bool
}{
	{
		name:    "valid type and severity",
		subject: "BUG/MEDIUM: config: add default location of path to the configuration file",
		wantErr: false,
	},
	{
		name:    "invalid type and ok severity err front",
		subject: "aaaBUG/MEDIUM: config: add default location of path to the configuration file",
		wantErr: true,
	},
	{
		name:    "invalid type and ok severity err back",
		subject: "BUG/MEDIUMaaa: config: add default location of path to the configuration file",
		wantErr: true,
	},
	{
		name:    "short subject",
		subject: "BUG/MEDIUM: config: default",
		wantErr: true,
	},
	{
		name:    "missing severity",
		subject: "BUG/: config: default implementation",
		wantErr: true,
	},
	{
		name:    "only severity",
		subject: "MINOR: config: default implementation",
		wantErr: false,
	},
	{
		name:    "wrong tag",
		subject: "WRONG: config: default implementation",
		wantErr: true,
	},
	{
		name:    "wrong severity",
		subject: "BUG/WRONG: config: default implementation",
		wantErr: true,
	},
	{
		name:    "double spaces",
		subject: "BUG/MEDIUM: config:  default implementation",
		wantErr: true,
	},
	{
		name:    "trailing spaces",
		subject: "BUG/MEDIUM: config: default implementation ",
		wantErr: true,
	},
	{
		name:    "unprocessed tags remain",
		subject: "BUG/MINOR: MAJOR: config: default implementation",
		wantErr: true,
	},
}
