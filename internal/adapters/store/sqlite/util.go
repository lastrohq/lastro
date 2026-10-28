package sqlite

import (
	"errors"
	"io/fs"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

func mustSub(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(err) // diretório embutido em tempo de compilação: só falha se o código estiver errado
	}
	return sub
}

func unico(err error) bool {
	var e *sqlite.Error
	return errors.As(err, &e) && (e.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE || e.Code() == sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY)
}

func boolInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
