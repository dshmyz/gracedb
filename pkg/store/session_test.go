package store

import (
	"testing"

	"github.com/dshmyz/gracedb/pkg/types"
)

func TestCreateSession(t *testing.T) {
	s := newTestStore(t)

	sess, err := s.CreateSession("test-session")
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	if sess.Name != "test-session" {
		t.Errorf("expected name 'test-session', got %q", sess.Name)
	}
	if sess.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestGetSession(t *testing.T) {
	s := newTestStore(t)
	sess, err := s.CreateSession("gettest")
	if err != nil {
		t.Fatal(err)
	}

	retrieved, err := s.GetSession(sess.ID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}
	if retrieved.Name != "gettest" {
		t.Errorf("expected name 'gettest', got %q", retrieved.Name)
	}
}

func TestAddMessageAndGetHistory(t *testing.T) {
	s := newTestStore(t)
	sess, err := s.CreateSession("chat")
	if err != nil {
		t.Fatal(err)
	}

	err = s.AddMessage(&types.Message{
		SessionID: sess.ID,
		Role:      "user",
		Content:   "Hello",
	})
	if err != nil {
		t.Fatalf("AddMessage failed: %v", err)
	}

	err = s.AddMessage(&types.Message{
		SessionID: sess.ID,
		Role:      "assistant",
		Content:   "Hi there!",
	})
	if err != nil {
		t.Fatalf("AddMessage failed: %v", err)
	}

	history, err := s.GetSessionHistory(sess.ID, 10)
	if err != nil {
		t.Fatalf("GetSessionHistory failed: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(history))
	}
	if history[0].Role != "user" {
		t.Errorf("expected first message role 'user', got %q", history[0].Role)
	}
	if history[1].Role != "assistant" {
		t.Errorf("expected second message role 'assistant', got %q", history[1].Role)
	}
}

func TestGetSessionHistoryLimit(t *testing.T) {
	s := newTestStore(t)
	sess, _ := s.CreateSession("limited")

	for i := 0; i < 5; i++ {
		s.AddMessage(&types.Message{
			SessionID: sess.ID,
			Role:      "user",
			Content:   string(rune('A' + i)),
		})
	}

	history, err := s.GetSessionHistory(sess.ID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 messages with limit, got %d", len(history))
	}
}

func TestDeleteSession(t *testing.T) {
	s := newTestStore(t)
	sess, _ := s.CreateSession("todelete")

	s.AddMessage(&types.Message{
		SessionID: sess.ID,
		Role:      "user",
		Content:   "bye",
	})

	err := s.DeleteSession(sess.ID)
	if err != nil {
		t.Fatalf("DeleteSession failed: %v", err)
	}

	// Session should be gone.
	_, err = s.GetSession(sess.ID)
	if err != types.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestDocumentCRUD(t *testing.T) {
	s := newTestStore(t)

	err := s.CreateDocument(&types.Document{
		ID:      "doc1",
		Source:  "test",
		Content: "test content",
	})
	if err != nil {
		t.Fatalf("CreateDocument failed: %v", err)
	}

	doc, err := s.GetDocument("doc1")
	if err != nil {
		t.Fatalf("GetDocument failed: %v", err)
	}
	if doc.Content != "test content" {
		t.Errorf("expected content 'test content', got %q", doc.Content)
	}

	err = s.DeleteDocument("doc1")
	if err != nil {
		t.Fatalf("DeleteDocument failed: %v", err)
	}

	_, err = s.GetDocument("doc1")
	if err != types.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestStats(t *testing.T) {
	s := newTestStore(t)
	s.CreateCollection("c1")
	s.CreateSession("s1")

	s.Upsert("c1", "d1", []float32{1, 2, 3}, "hello", nil, nil)

	stats, err := s.Stats()
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}
	if stats.CollectionCount != 1 {
		t.Errorf("expected 1 collection, got %d", stats.CollectionCount)
	}
	if stats.SessionCount != 1 {
		t.Errorf("expected 1 session, got %d", stats.SessionCount)
	}
	if stats.EmbeddingCount != 1 {
		t.Errorf("expected 1 embedding, got %d", stats.EmbeddingCount)
	}
}
