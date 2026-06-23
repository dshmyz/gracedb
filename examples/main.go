package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dshmyz/gracedb/pkg/gracedb"
	"github.com/dshmyz/gracedb/pkg/graph"
	"github.com/dshmyz/gracedb/pkg/types"
)

func main() {
	tmpDir, err := os.MkdirTemp("", "gracedb-demo-*")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "gracedb")

	// 1. Open the database.
	db, err := gracedb.Open(dbPath,
		gracedb.WithIndexType("hnsw"),
		gracedb.WithSimilarity("cosine"),
		gracedb.WithEmbedder(demoEmbedder{}),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	fmt.Println("=== gracedb Demo ===")

	// 2. Create a collection.
	coll, err := db.CreateCollection("articles")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("1. Created collection: %s\n", coll.Name)

	// 3. Insert vectors with content (for FTS).
	vectors := [][]float32{
		{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8},
		{0.5, 0.6, 0.7, 0.8, 0.1, 0.2, 0.3, 0.4},
		{0.9, 0.1, 0.5, 0.3, 0.7, 0.2, 0.8, 0.4},
	}
	contents := []string{
		"Go is a programming language designed for simplicity and reliability",
		"Badger is a fast key-value store written in Go",
		"Vector databases enable semantic search with embeddings",
	}
	docIDs := []string{"doc-1", "doc-2", "doc-3"}

	err = db.UpsertBatch("articles", vectors, contents, docIDs, nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("2. Inserted %d embeddings\n", len(vectors))

	// 4. Vector similarity search.
	query := []float32{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8}
	results, err := db.Search("articles", query, types.SearchOptions{
		TopK:            2,
		UseVectorSearch: true,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("3. Vector search (top 2): found %d results\n", len(results))
	for i, r := range results {
		fmt.Printf("   %d. ID=%s, Score=%.4f, Content=%q\n", i+1, r.ID, r.Score, r.Content)
	}

	// 5. Full-text search.
	ftsResults, err := db.SearchFTSWithContent("articles", "Go programming", 2)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("4. FTS search: found %d results\n", len(ftsResults))

	// 6. Quick interface.
	q := db.Quick()
	ctx := context.Background()
	quickID, err := q.AddToCollection(ctx, "articles", []float32{0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9}, "Quick demo")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("5. Quick.Add: %s\n", quickID)

	// 7. Session management.
	sess, err := db.CreateSession("demo-chat")
	if err != nil {
		log.Fatal(err)
	}
	_ = db.AddMessage(&types.Message{SessionID: sess.ID, Role: "user", Content: "What is gracedb?"})
	_ = db.AddMessage(&types.Message{SessionID: sess.ID, Role: "assistant", Content: "gracedb is a vector database built on Badger KV store"})
	history, _ := db.GetSessionHistory(sess.ID, 10)
	fmt.Printf("6. Session '%s': %d messages\n", sess.Name, len(history))

	// 8. Knowledge storage.
	_, err = db.SaveKnowledge("articles", "k1", "Go Language", "Go is a statically typed, compiled programming language designed at Google.", types.KnowledgeSaveRequest{Content: "Go is a statically typed, compiled programming language designed at Google.", Title: "Go Language", Collection: "articles"})
	if err != nil {
		log.Printf("SaveKnowledge: %v", err)
	} else {
		fmt.Println("7. Saved knowledge item")
	}

	// 9. Semantic Agent Memory.
	runMemoryDemo(db)

	// 10. Property graph.
	g := db.Graph()
	_ = g.UpsertNode(&graph.GraphNode{ID: "node-alice", Type: "person", Labels: []string{"person"}})
	_ = g.UpsertNode(&graph.GraphNode{ID: "node-bob", Type: "person", Labels: []string{"person"}})
	_ = g.UpsertEdge(&graph.GraphEdge{ID: "edge-1", FromNodeID: "node-alice", ToNodeID: "node-bob", Type: "knows", Weight: 1.0})
	fmt.Println("9. Property graph: added 2 nodes + 1 edge")

	// 11. Backup.
	backupPath := filepath.Join(tmpDir, "backup.db")
	if err := db.Backup(backupPath); err != nil {
		log.Printf("Backup: %v", err)
	} else {
		fmt.Println("10. Backup created")
	}

	// 12. Stats.
	stats, err := db.Stats()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("11. Stats: collections=%d, embeddings=%d, sessions=%d\n",
		stats.CollectionCount, stats.EmbeddingCount, stats.SessionCount)

	fmt.Println("\nDemo completed successfully!")
}

func runMemoryDemo(db *gracedb.DB) {
	userID := "user-123"
	save := func(req types.MemorySaveRequest) {
		if _, err := db.SaveMemory(req); err != nil {
			log.Fatalf("SaveMemory(%s): %v", req.MemoryID, err)
		}
	}

	save(types.MemorySaveRequest{
		MemoryID:   "mem-go",
		Content:    "User prefers Go for backend services.",
		Scope:      types.MemoryScopeUser,
		UserID:     userID,
		Namespace:  "preferences",
		Importance: 0.8,
	})
	save(types.MemorySaveRequest{
		MemoryID:   "mem-tea",
		Content:    "User likes green tea in the afternoon.",
		Scope:      types.MemoryScopeUser,
		UserID:     userID,
		Namespace:  "preferences",
		Importance: 0.3,
	})
	save(types.MemorySaveRequest{
		MemoryID:   "mem-other-user",
		Content:    "Other user prefers Python notebooks.",
		Scope:      types.MemoryScopeUser,
		UserID:     "other-user",
		Namespace:  "preferences",
		Importance: 1.0,
	})

	fmt.Println("8. Semantic memory: saved scoped user memories")
	printMemorySearch(db, "semantic query", types.MemorySearchRequest{
		Query:     "backend language preference",
		Scope:     types.MemoryScopeUser,
		UserID:    userID,
		Namespace: "preferences",
		TopK:      3,
	})
	printMemorySearch(db, "custom lexical+importance weights", types.MemorySearchRequest{
		Query:            "green tea",
		Scope:            types.MemoryScopeUser,
		UserID:           userID,
		Namespace:        "preferences",
		TopK:             3,
		SemanticWeight:   0,
		LexicalWeight:    0.80,
		ImportanceWeight: 0.20,
		RecencyWeight:    0,
	})
	printMemorySearch(db, "short recency half-life", types.MemorySearchRequest{
		Query:           "preference",
		Scope:           types.MemoryScopeUser,
		UserID:          userID,
		Namespace:       "preferences",
		TopK:            3,
		LexicalWeight:   0.50,
		RecencyWeight:   0.50,
		RecencyHalfLife: time.Second,
	})

	updated := "User now prefers Rust for backend services."
	if _, err := db.UpdateMemory(types.MemoryUpdateRequest{
		MemoryID: "mem-go",
		Content:  &updated,
	}); err != nil {
		log.Fatalf("UpdateMemory: %v", err)
	}
	printMemorySearch(db, "after update", types.MemorySearchRequest{
		Query:     "rust backend",
		Scope:     types.MemoryScopeUser,
		UserID:    userID,
		Namespace: "preferences",
		TopK:      2,
	})

	if err := db.DeleteMemory("mem-tea"); err != nil {
		log.Fatalf("DeleteMemory: %v", err)
	}
	if _, err := db.GetMemory("mem-tea"); err != nil {
		fmt.Printf("   Deleted mem-tea; GetMemory now returns: %v\n", err)
	}
}

func printMemorySearch(db *gracedb.DB, label string, req types.MemorySearchRequest) {
	resp, err := db.SearchMemory(req)
	if err != nil {
		log.Fatalf("SearchMemory(%s): %v", label, err)
	}
	fmt.Printf("   Memory search [%s]: %d results\n", label, len(resp.Results))
	for i, hit := range resp.Results {
		fmt.Printf("      %d. id=%s final=%.3f sem=%.3f lex=%.3f imp=%.3f rec=%.3f content=%q\n",
			i+1,
			hit.Memory.ID,
			hit.FinalScore,
			hit.SemanticScore,
			hit.LexicalScore,
			hit.ImportanceScore,
			hit.RecencyScore,
			hit.Memory.Content,
		)
	}
}

type demoEmbedder struct{}

func (demoEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	lower := strings.ToLower(text)
	vec := []float32{0.01, 0.01, 0.01, 0.01}
	if strings.Contains(lower, "go") || strings.Contains(lower, "backend") || strings.Contains(lower, "language") {
		vec[0] = 1
	}
	if strings.Contains(lower, "tea") || strings.Contains(lower, "afternoon") {
		vec[1] = 1
	}
	if strings.Contains(lower, "python") || strings.Contains(lower, "notebook") {
		vec[2] = 1
	}
	if strings.Contains(lower, "rust") {
		vec[3] = 1
	}
	return vec, nil
}

func (e demoEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, text := range texts {
		vec, err := e.Embed(ctx, text)
		if err != nil {
			return nil, err
		}
		out[i] = vec
	}
	return out, nil
}

func (demoEmbedder) Dimension() int {
	return 4
}
