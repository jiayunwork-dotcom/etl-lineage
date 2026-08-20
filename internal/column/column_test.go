package column

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFieldRefString(t *testing.T) {
	f := FieldRef{Table: "orders", Column: "amount"}
	if s := f.String(); s != "orders.amount" {
		t.Errorf("String() = %q", s)
	}
}

func TestParseFieldRef(t *testing.T) {
	f, err := ParseFieldRef("users.email")
	if err != nil {
		t.Fatal(err)
	}
	if f.Table != "users" || f.Column != "email" {
		t.Errorf("parsed = %+v", f)
	}
}

func TestParseFieldRefInvalid(t *testing.T) {
	_, err := ParseFieldRef("noperiod")
	if err == nil {
		t.Error("expected error for invalid field ref")
	}
}

func TestLineageMapSourcesAndTargets(t *testing.T) {
	lm := NewLineageMap()
	lm.Add(Mapping{
		Source: FieldRef{Table: "raw", Column: "price"},
		Target: FieldRef{Table: "fact", Column: "amount"},
		Kind:   MapDirect,
	})
	lm.Add(Mapping{
		Source: FieldRef{Table: "raw", Column: "qty"},
		Target: FieldRef{Table: "fact", Column: "quantity"},
		Kind:   MapDirect,
	})
	lm.Add(Mapping{
		Source: FieldRef{Table: "fact", Column: "amount"},
		Target: FieldRef{Table: "report", Column: "total"},
		Kind:   MapAggregate,
	})

	// sources of fact.amount
	sources := lm.SourcesOf(FieldRef{Table: "fact", Column: "amount"})
	if len(sources) != 1 || sources[0].Source.Column != "price" {
		t.Errorf("sources of fact.amount = %+v", sources)
	}

	// targets of raw.price
	targets := lm.TargetsOf(FieldRef{Table: "raw", Column: "price"})
	if len(targets) != 1 || targets[0].Target.Column != "amount" {
		t.Errorf("targets of raw.price = %+v", targets)
	}
}

func TestTransitiveSources(t *testing.T) {
	lm := NewLineageMap()
	lm.Add(Mapping{
		Source: FieldRef{Table: "src", Column: "a"},
		Target: FieldRef{Table: "mid", Column: "b"},
		Kind:   MapDirect,
	})
	lm.Add(Mapping{
		Source: FieldRef{Table: "mid", Column: "b"},
		Target: FieldRef{Table: "dst", Column: "c"},
		Kind:   MapTransform,
	})

	sources := lm.TransitiveSourcesOf(FieldRef{Table: "dst", Column: "c"})
	if len(sources) != 2 {
		t.Fatalf("transitive sources = %d, want 2", len(sources))
	}
}

func TestTransitiveTargets(t *testing.T) {
	lm := NewLineageMap()
	lm.Add(Mapping{
		Source: FieldRef{Table: "a", Column: "x"},
		Target: FieldRef{Table: "b", Column: "y"},
		Kind:   MapDirect,
	})
	lm.Add(Mapping{
		Source: FieldRef{Table: "b", Column: "y"},
		Target: FieldRef{Table: "c", Column: "z"},
		Kind:   MapDirect,
	})

	targets := lm.TransitiveTargetsOf(FieldRef{Table: "a", Column: "x"})
	if len(targets) != 2 {
		t.Fatalf("transitive targets = %d, want 2", len(targets))
	}
}

func TestLineageMapTables(t *testing.T) {
	lm := NewLineageMap()
	lm.Add(Mapping{
		Source: FieldRef{Table: "alpha", Column: "x"},
		Target: FieldRef{Table: "beta", Column: "y"},
		Kind:   MapDirect,
	})
	tables := lm.Tables()
	if len(tables) != 2 {
		t.Errorf("tables = %v", tables)
	}
}

func TestLineageMapValidate(t *testing.T) {
	lm := NewLineageMap()
	lm.Add(Mapping{
		Source: FieldRef{Table: "", Column: "x"},
		Target: FieldRef{Table: "b", Column: "y"},
		Kind:   MapDirect,
	})
	issues := lm.Validate()
	if len(issues) != 1 {
		t.Errorf("issues = %d, want 1", len(issues))
	}
}

func TestSaveAndLoadLineageMap(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "col_lineage.json")

	lm := NewLineageMap()
	lm.Add(Mapping{
		Source: FieldRef{Table: "src", Column: "id"},
		Target: FieldRef{Table: "dst", Column: "source_id"},
		Kind:   MapDirect,
		Expr:   "CAST(id AS INT)",
	})

	if err := SaveLineageMap(path, lm); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadLineageMap(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Len() != 1 {
		t.Fatalf("loaded len = %d", loaded.Len())
	}
	m := loaded.AllMappings()[0]
	if m.Source.Table != "src" || m.Target.Column != "source_id" {
		t.Errorf("loaded mapping = %+v", m)
	}
}

func TestLoadLineageMapNotFound(t *testing.T) {
	_, err := LoadLineageMap("/nonexistent/path.json")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestMergeLineageMaps(t *testing.T) {
	lm1 := NewLineageMap()
	lm1.Add(Mapping{Source: FieldRef{"a", "x"}, Target: FieldRef{"b", "y"}, Kind: MapDirect})

	lm2 := NewLineageMap()
	lm2.Add(Mapping{Source: FieldRef{"a", "x"}, Target: FieldRef{"b", "y"}, Kind: MapDirect}) // dup
	lm2.Add(Mapping{Source: FieldRef{"c", "z"}, Target: FieldRef{"d", "w"}, Kind: MapTransform})

	merged := MergeLineageMaps(lm1, lm2)
	if merged.Len() != 2 {
		t.Errorf("merged len = %d, want 2 (deduped)", merged.Len())
	}
}

func TestColumnsOfTable(t *testing.T) {
	lm := NewLineageMap()
	lm.Add(Mapping{Source: FieldRef{"t", "a"}, Target: FieldRef{"t", "b"}, Kind: MapDirect})
	lm.Add(Mapping{Source: FieldRef{"t", "c"}, Target: FieldRef{"u", "d"}, Kind: MapDirect})

	cols := lm.ColumnsOfTable("t")
	if len(cols) != 3 {
		t.Errorf("cols = %v, want 3", cols)
	}
}

func init() {
	// suppress unused import warning
	_ = os.Remove
}
