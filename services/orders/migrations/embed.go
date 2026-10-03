// Package migrations embute o SQL do orders-service no binário.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
