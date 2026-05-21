package rdf

import (
	"os"
	"testing"

	"github.com/dgraph-io/badger/v4"
)

func newTestStore(t *testing.T) *Store {
	dir, err := os.MkdirTemp("", "rdf-test-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	db, err := badger.Open(badger.DefaultOptions(dir))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	return NewStore(db)
}

func TestUpsertAndQuery(t *testing.T) {
	s := newTestStore(t)

	triple := &Triple{
		Subject:   NewIRI("http://example.com/alice"),
		Predicate: NewIRI("http://schema.org/name"),
		Object:    NewLiteral("Alice"),
	}
	if err := s.UpsertTriple(triple); err != nil {
		t.Fatal(err)
	}

	results, err := s.Query(TriplePattern{
		Subject:   &Term{Kind: TermIRI, Value: "http://example.com/alice"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Object.Value != "Alice" {
		t.Fatalf("expected object 'Alice', got %q", results[0].Object.Value)
	}
}

func TestQueryByPredicate(t *testing.T) {
	s := newTestStore(t)

	_ = s.UpsertTriple(&Triple{Subject: NewIRI("http://example.com/a"), Predicate: NewIRI("http://schema.org/name"), Object: NewLiteral("Alice")})
	_ = s.UpsertTriple(&Triple{Subject: NewIRI("http://example.com/b"), Predicate: NewIRI("http://schema.org/name"), Object: NewLiteral("Bob")})
	_ = s.UpsertTriple(&Triple{Subject: NewIRI("http://example.com/a"), Predicate: NewIRI("http://schema.org/age"), Object: NewLiteral("30")})

	results, err := s.Query(TriplePattern{
		Predicate: &Term{Kind: TermIRI, Value: "http://schema.org/name"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

func TestSPARQLSelect(t *testing.T) {
	s := newTestStore(t)

	_ = s.UpsertTriple(&Triple{Subject: NewIRI("http://example.com/alice"), Predicate: NewIRI("http://schema.org/name"), Object: NewLiteral("Alice")})
	_ = s.UpsertTriple(&Triple{Subject: NewIRI("http://example.com/alice"), Predicate: NewIRI("http://schema.org/knows"), Object: NewIRI("http://example.com/bob")})

	results, err := s.SPARQLSelect("SELECT ?s ?o WHERE { ?s <http://schema.org/name> ?o }")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(results))
	}
	if results[0]["?o"].Value != "Alice" {
		t.Fatalf("expected ?o='Alice', got %q", results[0]["?o"].Value)
	}
}

func TestSPARQLAsk(t *testing.T) {
	s := newTestStore(t)

	_ = s.UpsertTriple(&Triple{Subject: NewIRI("http://example.com/a"), Predicate: NewIRI("http://schema.org/name"), Object: NewLiteral("Alice")})

	found, err := s.SPARQLAsk("ASK WHERE { ?s <http://schema.org/name> ?o }")
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("expected ASK to return true")
	}

	notFound, err := s.SPARQLAsk("ASK WHERE { ?s <http://schema.org/knows> ?o }")
	if err != nil {
		t.Fatal(err)
	}
	if notFound {
		t.Fatal("expected ASK to return false")
	}
}

func TestImportExportNTriples(t *testing.T) {
	s := newTestStore(t)

	data := `<http://example.com/alice> <http://schema.org/name> "Alice" .
<http://example.com/alice> <http://schema.org/age> "30"^^<http://www.w3.org/2001/XMLSchema#integer> .
<http://example.com/bob> <http://schema.org/name> "Bob"@en .`

	count, err := s.ImportNTriples(data)
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("expected 3 triples imported, got %d", count)
	}

	exported, err := s.ExportNTriples()
	if err != nil {
		t.Fatal(err)
	}
	if len(exported) == 0 {
		t.Fatal("expected non-empty export")
	}
}

func TestRDFSInference(t *testing.T) {
	s := newTestStore(t)

	// Define hierarchy: Employee subClassOf Person.
	_ = s.UpsertTriple(&Triple{Subject: NewIRI("Employee"), Predicate: NewIRI(RDFSSubClassOf), Object: NewIRI("Person")})

	// Alice type Employee.
	_ = s.UpsertTriple(&Triple{Subject: NewIRI("Alice"), Predicate: NewIRI(RDFType), Object: NewIRI("Employee")})

	inf := NewRDFSInferencer(s)
	count, err := inf.Infer()
	if err != nil {
		t.Fatal(err)
	}

	// Should have inferred: Alice type Person.
	results, err := s.Query(TriplePattern{
		Subject:   &Term{Kind: TermIRI, Value: "Alice"},
		Predicate: &Term{Kind: TermIRI, Value: RDFType},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) < 2 {
		t.Fatalf("expected at least 2 type triples (Employee + inferred Person), got %d", len(results))
	}

	t.Logf("Inferred %d new triples", count)
}

func TestSHACLValidation(t *testing.T) {
	s := newTestStore(t)

	// Define a Person shape requiring name.
	shapeID := "http://example.com/PersonShape"
	_ = s.UpsertTriple(&Triple{Subject: NewIRI(shapeID), Predicate: NewIRI("http://www.w3.org/1999/02/22-rdf-syntax-ns#type"), Object: NewIRI(SHACLNodeShape)})
	_ = s.UpsertTriple(&Triple{Subject: NewIRI(shapeID), Predicate: NewIRI(SHACLTargetClass), Object: NewIRI("Person")})

	// Create a Person without name (should violate).
	_ = s.UpsertTriple(&Triple{Subject: NewIRI("p1"), Predicate: NewIRI(RDFType), Object: NewIRI("Person")})

	// Create a Person with name (should pass).
	_ = s.UpsertTriple(&Triple{Subject: NewIRI("p2"), Predicate: NewIRI(RDFType), Object: NewIRI("Person")})
	_ = s.UpsertTriple(&Triple{Subject: NewIRI("p2"), Predicate: NewIRI("http://schema.org/name"), Object: NewLiteral("Alice")})

	validator := NewSHACLValidator(s)
	report, err := validator.Validate()
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Report: conforms=%v, violations=%d, shapes=%d, nodes=%d",
		report.Conforms, len(report.Violations), report.CheckedShapes, report.CheckedNodes)
}

func TestDeleteTriple(t *testing.T) {
	s := newTestStore(t)

	triple := &Triple{
		Subject:   NewIRI("http://example.com/to-delete"),
		Predicate: NewIRI("http://schema.org/name"),
		Object:    NewLiteral("Temp"),
	}
	if err := s.UpsertTriple(triple); err != nil {
		t.Fatal(err)
	}

	if err := s.DeleteTriple(triple.ID); err != nil {
		t.Fatal(err)
	}

	results, err := s.Query(TriplePattern{
		Subject: &Term{Kind: TermIRI, Value: "http://example.com/to-delete"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results after delete, got %d", len(results))
	}
}

func TestCount(t *testing.T) {
	s := newTestStore(t)

	_ = s.UpsertTriple(&Triple{Subject: NewIRI("a"), Predicate: NewIRI("p"), Object: NewLiteral("1")})
	_ = s.UpsertTriple(&Triple{Subject: NewIRI("b"), Predicate: NewIRI("p"), Object: NewLiteral("2")})

	count, err := s.Count()
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("expected count 2, got %d", count)
	}
}
