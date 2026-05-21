package store

import (
	"github.com/dshmyz/gracedb/pkg/types"
)

// Store defines the interface for the vector storage backend.
type Store interface {
	// Lifecycle
	Close() error

	// Collections
	CreateCollection(name string) (*types.Collection, error)
	GetCollection(name string) (*types.Collection, error)
	ListCollections() ([]*types.Collection, error)
	DeleteCollection(name string) error

	// Embeddings
	Upsert(collectionName string, docID string, vector []float32, content string, metadata map[string]string, acl []string) (string, error)
	UpsertBatch(collectionName string, vectors [][]float32, contents []string, docIDs []string, metadata []map[string]string) ([]string, error)
	GetEmbedding(collectionID, embID string, includeVector bool) (*types.Embedding, error)
	DeleteEmbedding(collectionID, embID string) error
	DeleteByDocID(collectionID, docID string) error
	DeleteBatch(collectionID string, ids []string) error

	// Search
	Search(collectionName string, query []float32, opts types.SearchOptions) ([]types.ScoredEmbedding, error)
	Aggregate(collectionName string, metadataKey string, aggType AggregationType) (*AggregationResult, error)
	SearchGeo(collectionName string, query GeoQuery, opts types.SearchOptions) ([]types.ScoredEmbedding, error)

	// FTS
	IndexFTS(collectionID, embID, content string) error
	UnindexFTS(collectionID, embID string) error
	SearchFTS(collectionID string, query string) ([]string, error)
	SearchFTSWithContent(collectionID string, query string, topK int) ([]types.ScoredEmbedding, error)

	// Index management
	ReadVectors(collectionID string) (map[string][]float32, error)
	EmbeddingCount(collectionID string) (int, error)
	ListEmbeddingIDs(collectionID string) ([]string, error)
	LoadIndex(collectionName string) error
	SaveIndex(collectionName string) error

	// Documents
	CreateDocument(doc *types.Document) error
	GetDocument(id string) (*types.Document, error)
	DeleteDocument(id string) error

	// Sessions
	CreateSession(name string) (*types.Session, error)
	GetSession(id string) (*types.Session, error)
	AddMessage(msg *types.Message) error
	GetSessionHistory(sessionID string, limit int) ([]*types.Message, error)

	// Stats
	Stats() (types.StoreStats, error)

	// Knowledge
	SaveKnowledge(collectionName, knowledgeID, title, content string, req types.KnowledgeSaveRequest) (*types.KnowledgeRecord, error)
	GetKnowledge(collectionName, knowledgeID string) (*types.KnowledgeRecord, error)
	UpdateKnowledge(collectionName, knowledgeID string, req types.KnowledgeUpdateRequest) (*types.KnowledgeRecord, error)
	DeleteKnowledge(collectionName, knowledgeID string) error
	SearchKnowledge(collectionName, query string, topK int) (*types.KnowledgeSearchResponse, error)

	// Memory
	SaveMemory(req types.MemorySaveRequest) (*types.MemoryRecord, error)
	GetMemory(memoryID string) (*types.MemoryRecord, error)
	UpdateMemory(req types.MemoryUpdateRequest) (*types.MemoryRecord, error)
	DeleteMemory(memoryID string) error
	SearchMemory(req types.MemorySearchRequest) (*types.MemorySearchResponse, error)
}
