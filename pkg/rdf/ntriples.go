package rdf

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// parseNTriplesLine parses a single N-Triples line.
// Format: <subject> <predicate> <object> .
// or: <subject> <predicate> "literal" .
// or: <subject> <predicate> "literal"@lang .
// or: <subject> <predicate> "literal"^^<datatype> .
func parseNTriplesLine(line string) (*Triple, error) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return nil, nil
	}
	if !strings.HasSuffix(line, " .") {
		return nil, fmt.Errorf("n-triples line must end with ' .': %s", line)
	}
	line = strings.TrimSpace(line[:len(line)-2])

	tokens := parseNTriplesTokens(line)
	if len(tokens) < 3 {
		return nil, fmt.Errorf("expected 3 tokens, got %d", len(tokens))
	}

	subj, err := parseNTerm(tokens[0])
	if err != nil {
		return nil, fmt.Errorf("parse subject: %w", err)
	}
	pred, err := parseNTerm(tokens[1])
	if err != nil {
		return nil, fmt.Errorf("parse predicate: %w", err)
	}
	obj, err := parseNTerm(tokens[2])
	if err != nil {
		return nil, fmt.Errorf("parse object: %w", err)
	}

	return &Triple{
		ID:        tripleID(subj, pred, obj),
		Subject:   subj,
		Predicate: pred,
		Object:    obj,
	}, nil
}

func tripleID(s, p, o Term) string {
	h := sha256.Sum256([]byte(s.Value + "|" + p.Value + "|" + o.Value))
	return hex.EncodeToString(h[:8])
}

func parseNTriplesTokens(line string) []string {
	var tokens []string
	i := 0
	for i < len(line) {
		// Skip whitespace.
		for i < len(line) && line[i] == ' ' {
			i++
		}
		if i >= len(line) {
			break
		}

		if line[i] == '<' {
			// IRI.
			j := strings.Index(line[i:], ">")
			if j < 0 {
				break
			}
			tokens = append(tokens, line[i:i+j+1])
			i += j + 1
		} else if line[i] == '"' {
			// Literal.
			j := i + 1
			for j < len(line) {
				if line[j] == '"' && line[j-1] != '\\' {
					j++
					break
				}
				j++
			}
			// Check for language tag or datatype.
			if j < len(line) && line[j] == '@' {
				k := j + 1
				for k < len(line) && (line[k] >= 'a' && line[k] <= 'z' || line[k] >= 'A' && line[k] <= 'Z' || line[k] >= '0' && line[k] <= '9' || line[k] == '-') {
					k++
				}
				tokens = append(tokens, line[i:k])
				i = k
			} else if j < len(line) && line[j:j+2] == "^^" {
				k := j + 2
				if k < len(line) && line[k] == '<' {
					end := strings.Index(line[k:], ">")
					if end >= 0 {
						k += end + 1
						tokens = append(tokens, line[i:k])
						i = k
					} else {
						tokens = append(tokens, line[i:])
						i = len(line)
					}
				} else {
					tokens = append(tokens, line[i:])
					i = len(line)
				}
			} else {
				tokens = append(tokens, line[i:j])
				i = j
			}
		} else {
			// Unknown token.
			j := i
			for j < len(line) && line[j] != ' ' {
				j++
			}
			tokens = append(tokens, line[i:j])
			i = j
		}
	}
	return tokens
}

func parseNTerm(s string) (Term, error) {
	if strings.HasPrefix(s, "<") && strings.HasSuffix(s, ">") {
		return NewIRI(s[1 : len(s)-1]), nil
	}
	if strings.HasPrefix(s, "\"") {
		// Remove quotes.
		content := s[1:]
		if strings.HasSuffix(content, "\"") {
			content = content[:len(content)-1]
		}

		// Check for language tag.
		if at := strings.Index(content, "\"@"); at > 0 {
			val := content[:at]
			lang := content[at+2:]
			return NewLangLiteral(val, lang), nil
		}

		// Check for datatype.
		if caret := strings.Index(content, "\"^^"); caret > 0 {
			val := content[:caret]
			dt := content[caret+3:]
			if strings.HasPrefix(dt, "<") && strings.HasSuffix(dt, ">") {
				dt = dt[1 : len(dt)-1]
			}
			return NewTypedLiteral(val, dt), nil
		}

		// Handle escaped quotes.
		val := strings.ReplaceAll(content, "\\\"", "\"")
		return NewLiteral(val), nil
	}
	return Term{}, fmt.Errorf("unknown term format: %s", s)
}

// formatNTriples formats a triple in N-Triples syntax.
func formatNTriples(t Triple) string {
	subj := formatNTerm(t.Subject)
	pred := formatNTerm(t.Predicate)
	obj := formatNTerm(t.Object)
	return fmt.Sprintf("%s %s %s .", subj, pred, obj)
}

func formatNTerm(t Term) string {
	switch t.Kind {
	case TermIRI:
		return "<" + t.Value + ">"
	case TermBNode:
		return "_:" + t.Value
	case TermLiteral:
		if t.Language != "" {
			return fmt.Sprintf("\"%s\"@%s", escapeLiteral(t.Value), t.Language)
		}
		if t.Datatype != "" {
			return fmt.Sprintf("\"%s\"^^<%s>", escapeLiteral(t.Value), t.Datatype)
		}
		return fmt.Sprintf("\"%s\"", escapeLiteral(t.Value))
	default:
		return fmt.Sprintf("\"%s\"", t.Value)
	}
}

func escapeLiteral(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	return s
}
