package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

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

	// 9. Memory storage.
	mem, err := db.SaveMemory(types.MemorySaveRequest{
		MemoryID: "mem-1",
		Content:  "User prefers Go over Python",
		Scope:    "user",
	})
	if err != nil {
		log.Printf("SaveMemory: %v", err)
	} else {
		fmt.Printf("8. Saved memory: %s\n", mem.ID)
	}

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
