package bootstrap

import (
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

const module = "github.com/remodov/marketplace-system-go/services/catalog"

var forbiddenInCore = []string{
	"github.com/go-chi/chi", "github.com/jackc/pgx", "github.com/pressly/goose",
	"github.com/golang-jwt", "github.com/MicahParks", "net/http", "database/sql",
}

func TestCoreDependsOnlyOnCoreAndStdlib(t *testing.T) {
	pkgs := load(t, "../core/...")
	for _, p := range pkgs {
		for imp := range p.Imports {
			if strings.HasPrefix(imp, module+"/internal/adapter") || strings.HasPrefix(imp, module+"/internal/bootstrap") {
				t.Errorf("пакет ядра %s импортирует %s", p.PkgPath, imp)
			}
			for _, bad := range forbiddenInCore {
				if strings.HasPrefix(imp, bad) {
					t.Errorf("пакет ядра %s импортирует технологию %s", p.PkgPath, imp)
				}
			}
		}
	}
}

func TestInAdaptersDoNotImportOutAdapters(t *testing.T) {
	for _, p := range load(t, "../adapter/in/...") {
		for imp := range p.Imports {
			if strings.HasPrefix(imp, module+"/internal/adapter/out") {
				t.Errorf("входной адаптер %s импортирует выходной %s", p.PkgPath, imp)
			}
		}
	}
}

func TestOutAdaptersDoNotImportEachOther(t *testing.T) {
	for _, p := range load(t, "../adapter/out/...") {
		own := strings.SplitAfter(p.PkgPath, "/internal/adapter/out/")
		for imp := range p.Imports {
			if strings.HasPrefix(imp, module+"/internal/adapter/out/") && len(own) == 2 && !strings.HasPrefix(imp, module+"/internal/adapter/out/"+strings.SplitN(own[1], "/", 2)[0]) {
				t.Errorf("выходной адаптер %s импортирует соседний %s", p.PkgPath, imp)
			}
		}
	}
}

func load(t *testing.T, pattern string) []*packages.Package {
	t.Helper()
	pkgs, err := packages.Load(&packages.Config{Mode: packages.NeedName | packages.NeedImports}, pattern)
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) == 0 {
		t.Fatalf("по шаблону %s не найдено ни одного пакета", pattern)
	}
	return pkgs
}
