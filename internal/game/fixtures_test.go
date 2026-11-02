package game

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Золотые тесты лежат в репозитории frontend, рядом с движком на JS. В CI путь
// задаёт GF_FIXTURES; на своей машине достаточно положить репозитории рядом.
func fixturesDir(t *testing.T) string {
	t.Helper()
	if dir := os.Getenv("GF_FIXTURES"); dir != "" {
		return dir
	}
	dir := filepath.Join("..", "..", "..", "frontend", "fixtures", "cases")
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("фикстур нет в %s: положите frontend рядом с backend или задайте GF_FIXTURES", dir)
	}
	return dir
}

// fixture — один случай по формату frontend/fixtures/README.md.
type fixture struct {
	Name   string          `json:"name"`
	Cfg    json.RawMessage `json:"cfg"`
	Before json.RawMessage `json:"before"`
	Play   struct {
		Mode string `json:"mode"`
	} `json:"play"`
	After json.RawMessage `json:"after"`
}

func TestFixturesReadable(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(fixturesDir(t), "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("набор пуст: фикстуры не доехали")
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var c fixture
		if err := json.Unmarshal(data, &c); err != nil {
			t.Errorf("%s: %v", filepath.Base(f), err)
			continue
		}
		switch {
		case c.Name == "":
			t.Errorf("%s: нет name", filepath.Base(f))
		case len(c.Cfg) == 0 || len(c.Before) == 0 || len(c.After) == 0:
			t.Errorf("%s: нет cfg, before или after", filepath.Base(f))
		case c.Play.Mode != "round" && c.Play.Mode != "seq" && c.Play.Mode != "resign":
			t.Errorf("%s: неизвестный play.mode %q", filepath.Base(f), c.Play.Mode)
		}
	}
	t.Logf("фикстур: %d", len(files))
}
