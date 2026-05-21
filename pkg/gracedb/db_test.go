package gracedb

import (
	"testing"
)

func TestDB_OpenClose(t *testing.T) {
	db := testDB(t)
	if db == nil {
		t.Fatal("db is nil")
	}
	if db.Vector() == nil {
		t.Error("Vector() is nil")
	}
	if db.Graph() == nil {
		t.Error("Graph() is nil")
	}
	if db.RDF() == nil {
		t.Error("RDF() is nil")
	}
	if db.HasEmbedder() {
		t.Error("HasEmbedder() should be false without embedder")
	}
}

func TestDB_OpenWithEmbedder(t *testing.T) {
	db := testDB(t, WithEmbedder(newMockEmbedder(16)))
	if db == nil {
		t.Fatal("db is nil")
	}
	if !db.HasEmbedder() {
		t.Error("HasEmbedder() should be true")
	}
}

func TestDB_OpenWithOptions(t *testing.T) {
	db := testDB(t,
		WithIndexType("flat"),
		WithSimilarity("cosine"),
	)
	if db == nil {
		t.Fatal("db is nil")
	}
}

func TestDB_AccessorsReturnNonNil(t *testing.T) {
	db := testDB(t)
	if db.Vector() == nil {
		t.Error("Vector() should not be nil")
	}
	if db.Graph() == nil {
		t.Error("Graph() should not be nil")
	}
	if db.RDF() == nil {
		t.Error("RDF() should not be nil")
	}
}

func TestDB_CreateAndGetCollection(t *testing.T) {
	db := testDB(t)

	coll, err := db.CreateCollection("test_coll")
	if err != nil {
		t.Fatalf("create collection: %v", err)
	}
	if coll.Name != "test_coll" {
		t.Errorf("expected name test_coll, got %s", coll.Name)
	}
	if coll.EmbeddingCount != 0 {
		t.Errorf("expected 0 embeddings, got %d", coll.EmbeddingCount)
	}

	got, err := db.GetCollection("test_coll")
	if err != nil {
		t.Fatalf("get collection: %v", err)
	}
	if got.ID != coll.ID {
		t.Errorf("expected ID %s, got %s", coll.ID, got.ID)
	}
}

func TestDB_ListCollections(t *testing.T) {
	db := testDB(t)

	if _, err := db.CreateCollection("a"); err != nil {
		t.Fatalf("create a: %v", err)
	}
	if _, err := db.CreateCollection("b"); err != nil {
		t.Fatalf("create b: %v", err)
	}

	colls, err := db.ListCollections()
	if err != nil {
		t.Fatalf("list collections: %v", err)
	}
	if len(colls) != 2 {
		t.Errorf("expected 2 collections, got %d", len(colls))
	}
}

func TestDB_DeleteCollection(t *testing.T) {
	db := testDB(t)

	if _, err := db.CreateCollection("to_delete"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := db.DeleteCollection("to_delete"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err := db.GetCollection("to_delete")
	if err == nil {
		t.Error("expected error getting deleted collection")
	}
}

func TestDB_QuickReturnsQuick(t *testing.T) {
	db := testDB(t)
	q := db.Quick()
	if q == nil {
		t.Error("Quick() returned nil")
	}
}

func TestDB_GraphRAGToolsReturnsToolbox(t *testing.T) {
	db := testDB(t)
	tbx := db.GraphRAGTools()
	if tbx == nil {
		t.Error("GraphRAGTools() returned nil")
	}
}

func TestDB_NewMCPServer(t *testing.T) {
	db := testDB(t)
	srv := db.NewMCPServer("test", "1.0")
	if srv == nil {
		t.Error("NewMCPServer() returned nil")
	}
}

func TestDB_Stats(t *testing.T) {
	db := testDB(t)

	stats, err := db.Stats()
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if stats.CollectionCount != 0 {
		t.Errorf("expected 0 collections, got %d", stats.CollectionCount)
	}

	if _, err := db.CreateCollection("stats_test"); err != nil {
		t.Fatalf("create: %v", err)
	}

	stats, err = db.Stats()
	if err != nil {
		t.Fatalf("stats after create: %v", err)
	}
	if stats.CollectionCount < 1 {
		t.Errorf("expected >= 1 collection, got %d", stats.CollectionCount)
	}
}
