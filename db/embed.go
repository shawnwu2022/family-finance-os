package dbmigrations

import "embed"

// Files 包含 finance-core 二进制使用的版本化 SQL migrations。
//
//go:embed migrations/*.sql
var Files embed.FS
