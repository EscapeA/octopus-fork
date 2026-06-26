package db

import "testing"

func TestNormalizeMySQLDSN(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "standard DSN with tcp already",
			input: "root:pass@tcp(127.0.0.1:3306)/octopus",
			want:  "root:pass@tcp(127.0.0.1:3306)/octopus",
		},
		{
			name:  "mysql:// scheme with user pass",
			input: "mysql://root:pass@127.0.0.1:3306/octopus",
			want:  "root:pass@tcp(127.0.0.1:3306)/octopus",
		},
		{
			name:  "mysql:// scheme with existing query params",
			input: "mysql://root:pass@127.0.0.1:3306/octopus?charset=utf8mb4",
			want:  "root:pass@tcp(127.0.0.1:3306)/octopus?charset=utf8mb4",
		},
		{
			name:  "mysql:// scheme user only no password",
			input: "mysql://root@localhost:3306/mydb",
			want:  "root@tcp(localhost:3306)/mydb",
		},
		{
			name:  "password contains @ (uses last @ as separator)",
			input: "mysql://user:p@ss@host:3306/db",
			want:  "user:p@ss@tcp(host:3306)/db",
		},
		{
			name:  "no mysql:// prefix but missing tcp()",
			input: "root:pass@host:3306/octopus",
			want:  "root:pass@tcp(host:3306)/octopus",
		},
		{
			name:  "mysql:// no user, just host:port/db",
			input: "mysql://host:3306/dbname",
			want:  "tcp(host:3306)/dbname",
		},
		{
			name:  "mysql:// no user with query params",
			input: "mysql://host:3306/dbname?ssl=true",
			want:  "tcp(host:3306)/dbname?ssl=true",
		},
		{
			name:  "leading/trailing whitespace trimmed",
			input: "  mysql://root:pass@host:3306/db  ",
			want:  "root:pass@tcp(host:3306)/db",
		},
		{
			name:  "uppercase MYSQL:// scheme",
			input: "MYSQL://root:pass@host:3306/db",
			want:  "root:pass@tcp(host:3306)/db",
		},
		{
			name:  "unix socket not wrapped",
			input: "root:pass@unix(/tmp/mysql.sock)/db",
			want:  "root:pass@unix(/tmp/mysql.sock)/db",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeMySQLDSN(tc.input)
			if got != tc.want {
				t.Fatalf("normalizeMySQLDSN(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
