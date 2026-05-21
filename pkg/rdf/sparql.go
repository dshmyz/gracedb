package rdf

import (
	"fmt"
	"strings"
	"unicode"
)

// parseSimplifiedSPARQL parses a simplified SPARQL query.
// Supports: SELECT ?var WHERE { ?s ?p ?o . } and ASK WHERE { ?s ?p ?o }
func parseSimplifiedSPARQL(query string) ([]TriplePattern, []string, error) {
	query = strings.TrimSpace(query)
	upper := strings.ToUpper(query)

	var vars []string

	if strings.HasPrefix(upper, "SELECT") {
		// Extract variables from SELECT.
		whereIdx := strings.Index(query, "WHERE")
		if whereIdx < 0 {
			whereIdx = strings.Index(query, "{")
		}
		if whereIdx >= 0 {
			selectPart := strings.TrimSpace(query[7:whereIdx])
			for _, v := range strings.Fields(selectPart) {
				if strings.HasPrefix(v, "?") || strings.HasPrefix(v, "$") {
					vars = append(vars, strings.TrimPrefix(v, "$"))
				}
			}
		}
	} else if strings.HasPrefix(upper, "ASK") {
		// ASK has no variables to project.
	}

	// Extract WHERE clause.
	whereStart := strings.Index(query, "{")
	whereEnd := strings.LastIndex(query, "}")
	if whereStart < 0 || whereEnd <= whereStart {
		return nil, vars, fmt.Errorf("invalid SPARQL: missing WHERE { } block")
	}
	whereClause := query[whereStart+1 : whereEnd]

	patterns, err := parseTriplePatterns(whereClause)
	return patterns, vars, err
}

func parseTriplePatterns(clause string) ([]TriplePattern, error) {
	var patterns []TriplePattern
	clause = strings.TrimSpace(clause)

	// Split by '.' to get individual triple patterns.
	parts := splitByDot(clause)
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		p, err := parseTriplePattern(part)
		if err != nil {
			return nil, err
		}
		patterns = append(patterns, p)
	}
	return patterns, nil
}

func parseTriplePattern(s string) (TriplePattern, error) {
	tokens := tokenizeSPARQL(s)
	if len(tokens) < 3 {
		return TriplePattern{}, fmt.Errorf("invalid triple pattern: %s", s)
	}

	subj := parseTerm(tokens[0])
	pred := parseTerm(tokens[1])
	obj := parseTerm(tokens[2])

	// Variables (empty Kind) become nil in pattern (wildcard).
	var sp, pp, op *Term
	if subj.Kind != "" {
		sp = &subj
	}
	if pred.Kind != "" {
		pp = &pred
	}
	if obj.Kind != "" {
		op = &obj
	}

	return TriplePattern{
		Subject:   sp,
		Predicate: pp,
		Object:    op,
	}, nil
}

func tokenizeSPARQL(s string) []string {
	var tokens []string
	var current strings.Builder
	inQuotes := false
	inAngle := false

	for _, r := range s {
		switch {
		case r == '"':
			inQuotes = !inQuotes
			current.WriteRune(r)
		case r == '<':
			inAngle = true
			current.WriteRune(r)
		case r == '>':
			inAngle = false
			current.WriteRune(r)
		case unicode.IsSpace(r) && !inQuotes && !inAngle:
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens
}

func parseTerm(s string) Term {
	s = strings.TrimSpace(s)

	// Variable.
	if strings.HasPrefix(s, "?") || strings.HasPrefix(s, "$") {
		return Term{Kind: "", Value: s} // Empty kind = variable
	}

	// IRI.
	if strings.HasPrefix(s, "<") && strings.HasSuffix(s, ">") {
		return NewIRI(s[1 : len(s)-1])
	}

	// Prefixed name.
	if idx := strings.Index(s, ":"); idx > 0 {
		return Term{Kind: TermIRI, Value: s}
	}

	// Literal.
	if strings.HasPrefix(s, "\"") {
		val := strings.Trim(s, "\"")
		return NewLiteral(val)
	}

	return Term{Kind: TermIRI, Value: s}
}

func splitByDot(s string) []string {
	var parts []string
	var current strings.Builder
	inQuotes := false
	inAngle := false

	for _, r := range s {
		switch {
		case r == '"':
			inQuotes = !inQuotes
			current.WriteRune(r)
		case r == '<':
			inAngle = true
			current.WriteRune(r)
		case r == '>':
			inAngle = false
			current.WriteRune(r)
		case r == '.' && !inQuotes && !inAngle:
			parts = append(parts, current.String())
			current.Reset()
		default:
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts
}
