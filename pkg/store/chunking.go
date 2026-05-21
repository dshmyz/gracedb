package store

import (
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/dshmyz/gracedb/pkg/types"
)

// ChunkBySize splits content into chunks of approximately chunkSize characters
// with chunkOverlap overlap. Chunks break on sentence boundaries when possible.
func ChunkBySize(content string, chunkSize, chunkOverlap int) []types.Chunk {
	if chunkSize <= 0 {
		chunkSize = 500
	}
	if chunkOverlap < 0 || chunkOverlap >= chunkSize {
		chunkOverlap = chunkSize / 4
	}

	// Sentence boundary characters.
	breakChars := []rune{'。', '！', '？', '\n', '.', '!', '?'}

	var chunks []types.Chunk
	pos := 0
	contentLen := utf8.RuneCountInString(content)
	runes := []rune(content)

	for pos < contentLen {
		end := pos + chunkSize
		if end >= contentLen {
			end = contentLen
		} else {
			// Try to break at a sentence boundary.
			bestBreak := end
			for i := end - 1; i > pos+chunkSize/2; i-- {
				for _, bc := range breakChars {
					if runes[i] == bc {
						bestBreak = i + 1
						break
					}
				}
				if bestBreak != end {
					break
				}
			}
			end = bestBreak
		}

		chunkContent := string(runes[pos:end])
		chunks = append(chunks, types.Chunk{
			ID:      uuid.New().String(),
			Content: chunkContent,
			Start:   pos,
			End:     end,
		})

		// Move position forward with overlap.
		nextPos := end - chunkOverlap
		if nextPos <= pos {
			nextPos = end
		}
		pos = nextPos
	}

	return chunks
}

// LexicalSearchQueries generates multiple search queries from a base query,
// keywords, and alternate queries for expanded lexical search coverage.
func LexicalSearchQueries(query string, keywords, alternateQueries []string) []string {
	var queries []string

	if q := strings.TrimSpace(query); q != "" {
		queries = append(queries, q)
	}

	for _, kw := range keywords {
		if k := strings.TrimSpace(kw); k != "" {
			queries = append(queries, k)
		}
	}

	for _, alt := range alternateQueries {
		if a := strings.TrimSpace(alt); a != "" {
			queries = append(queries, a)
		}
	}

	if len(queries) == 0 && query != "" {
		queries = append(queries, query)
	}

	return queries
}
