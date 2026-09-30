package product_test

import (
	"context"
	"testing"
)

func TestMigrationsGiveEveryColumnTheTypeReads(t *testing.T) {
	rows, err := pool.Query(context.Background(),
		"SELECT column_name FROM information_schema.columns WHERE table_name = 'products'")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	have := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		have[name] = true
	}
	for _, want := range []string{"id", "title", "price", "stock", "reserved", "version"} {
		if !have[want] {
			t.Fatalf("после миграций в таблице products нет колонки %s", want)
		}
	}
}

func TestTrigramIndexIsInPlace(t *testing.T) {
	var name string
	err := pool.QueryRow(context.Background(),
		"SELECT indexname FROM pg_indexes WHERE tablename = 'products' AND indexdef LIKE '%gin_trgm_ops%'").Scan(&name)
	if err != nil {
		t.Fatalf("триграммного gin-индекса по title нет: %v", err)
	}
}
