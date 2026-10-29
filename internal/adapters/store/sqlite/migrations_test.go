package sqlite

import (
	"path/filepath"
	"testing"

	"github.com/pressly/goose/v3"
)

// A migration da cobr refaz a tabela txids: os txids que já existiam continuam
// valendo, na ida e na volta.
func TestMigrationDaCobrPreservaOsTxids(t *testing.T) {
	ctx := t.Context()
	s, err := Open(ctx, filepath.Join(t.TempDir(), "lastro.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	p, err := goose.NewProvider(goose.DialectSQLite3, s.db, mustSub(migrations, "migrations"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.DownTo(ctx, 202611010900); err != nil {
		t.Fatalf("descendo para antes da cobr: %v", err)
	}
	exec := func(sql string) error { _, err := s.db.ExecContext(ctx, sql); return err }
	for _, sql := range []string{
		`INSERT INTO txids (txid, tipo) VALUES ('txidantigo00000000000000001', 'cob')`,
	} {
		if err := exec(sql); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	if _, err := p.Up(ctx); err != nil {
		t.Fatalf("subindo: %v", err)
	}
	if err := exec(`INSERT INTO txids (txid, tipo) VALUES ('txidantigo00000000000000001', 'cobr')`); err == nil {
		t.Fatal("o txid de antes da migration deixou de ser único")
	}
	if err := exec(`INSERT INTO txids (txid, tipo) VALUES ('txidnovo0000000000000000001', 'cobr')`); err != nil {
		t.Fatalf("txids não aceita cobr: %v", err)
	}
	if _, err := p.DownTo(ctx, 202611010900); err != nil {
		t.Fatalf("descendo de novo: %v", err)
	}
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM txids`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("txids depois de descer: %d, %v", n, err)
	}
}
