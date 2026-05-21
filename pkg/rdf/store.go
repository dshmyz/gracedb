// Package rdf provides RDF triple store, SPARQL SELECT/ASK, RDFS inference,
// and SHACL-lite validation on top of Badger.
package rdf

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/dgraph-io/badger/v4"
)

const (
	TermIRI      = "iri"
	TermLiteral  = "literal"
	TermBNode    = "bnode"

	triplePrefix  = "rdf:t:"
	subjIndex     = "rdf:s:"
	predIndex     = "rdf:p:"
	objIndex      = "rdf:o:"
	nsPrefix      = "rdf:ns:"
)

// Term is an RDF term.
type Term struct {
	Kind     string `json:"kind"`
	Value    string `json:"value"`
	Datatype string `json:"datatype,omitempty"`
	Language string `json:"language,omitempty"`
}

// Triple is an RDF triple.
type Triple struct {
	ID        string `json:"id"`
	Subject   Term   `json:"subject"`
	Predicate Term   `json:"predicate"`
	Object    Term   `json:"object"`
	Graph     string `json:"graph,omitempty"`
	Inferred  bool   `json:"inferred,omitempty"`
}

// TriplePattern is a pattern for querying triples. Nil = wildcard.
type TriplePattern struct {
	Subject   *Term
	Predicate *Term
	Object    *Term
	Graph     string
	Inferred  *bool
	Limit     int
}

// NewIRI creates an IRI term.
func NewIRI(v string) Term {
	return Term{Kind: TermIRI, Value: strings.Trim(v, "<>")}
}

// NewLiteral creates a literal term.
func NewLiteral(v string) Term {
	return Term{Kind: TermLiteral, Value: v}
}

// NewTypedLiteral creates a typed literal.
func NewTypedLiteral(v, datatype string) Term {
	return Term{Kind: TermLiteral, Value: v, Datatype: datatype}
}

// NewLangLiteral creates a language-tagged literal.
func NewLangLiteral(v, lang string) Term {
	return Term{Kind: TermLiteral, Value: v, Language: lang}
}

// Store is an RDF triple store on Badger.
type Store struct {
	db     *badger.DB
	namespaces map[string]string
}

// NewStore creates an RDF store.
func NewStore(db *badger.DB) *Store {
	s := &Store{
		db:         db,
		namespaces: make(map[string]string),
	}
	// Register builtin namespaces.
	s.namespaces["rdf"] = "http://www.w3.org/1999/02/22-rdf-syntax-ns#"
	s.namespaces["rdfs"] = "http://www.w3.org/2000/01/rdf-schema#"
	s.namespaces["xsd"] = "http://www.w3.org/2001/XMLSchema#"
	s.namespaces["owl"] = "http://www.w3.org/2002/07/owl#"
	s.namespaces["schema"] = "https://schema.org/"
	s.namespaces["foaf"] = "http://xmlns.com/foaf/0.1/"
	return s
}

// UpsertTriple inserts or replaces a triple.
func (s *Store) UpsertTriple(t *Triple) error {
	if t.ID == "" {
		t.ID = hashTriple(t)
	}

	return s.db.Update(func(txn *badger.Txn) error {
		data, err := json.Marshal(t)
		if err != nil {
			return err
		}

		key := []byte(triplePrefix + t.ID)
		if err := txn.Set(key, data); err != nil {
			return err
		}

		// Indexes for fast lookup.
		if t.Subject.Kind == TermIRI {
			_ = txn.Set([]byte(subjIndex+t.Subject.Value+":"+t.ID), nil)
		}
		if t.Predicate.Kind == TermIRI {
			_ = txn.Set([]byte(predIndex+t.Predicate.Value+":"+t.ID), nil)
		}
		if t.Object.Kind == TermIRI || t.Object.Kind == TermLiteral {
			_ = txn.Set([]byte(objIndex+termKey(t.Object)+":"+t.ID), nil)
		}
		return nil
	})
}

// DeleteTriple removes a triple by ID.
func (s *Store) DeleteTriple(id string) error {
	return s.db.Update(func(txn *badger.Txn) error {
		key := []byte(triplePrefix + id)
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		var t Triple
		err = item.Value(func(val []byte) error {
			return json.Unmarshal(val, &t)
		})
		if err != nil {
			return err
		}

		if err := txn.Delete(key); err != nil {
			return err
		}
		// Delete indexes.
		if t.Subject.Kind == TermIRI {
			_ = txn.Delete([]byte(subjIndex + t.Subject.Value + ":" + id))
		}
		if t.Predicate.Kind == TermIRI {
			_ = txn.Delete([]byte(predIndex + t.Predicate.Value + ":" + id))
		}
		if t.Object.Kind == TermIRI || t.Object.Kind == TermLiteral {
			_ = txn.Delete([]byte(objIndex + termKey(t.Object) + ":" + id))
		}
		return nil
	})
}

// Query returns triples matching a pattern.
func (s *Store) Query(pattern TriplePattern) ([]Triple, error) {
	var results []Triple

	err := s.db.View(func(txn *badger.Txn) error {
		// Determine best index to use.
		if pattern.Subject != nil && pattern.Subject.Kind == TermIRI {
			prefix := subjIndex + pattern.Subject.Value + ":"
			return s.scanIndex(txn, prefix, pattern, &results)
		}
		if pattern.Predicate != nil && pattern.Predicate.Kind == TermIRI {
			prefix := predIndex + pattern.Predicate.Value + ":"
			return s.scanIndex(txn, prefix, pattern, &results)
		}
		if pattern.Object != nil {
			prefix := objIndex + termKey(*pattern.Object) + ":"
			return s.scanIndex(txn, prefix, pattern, &results)
		}

		// Full scan.
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte(triplePrefix)
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				var t Triple
				if err := json.Unmarshal(val, &t); err != nil {
					return err
				}
				if matchesPattern(t, pattern) {
					results = append(results, t)
				}
				return nil
			})
			if err != nil {
				return err
			}
			if pattern.Limit > 0 && len(results) >= pattern.Limit {
				break
			}
		}
		return nil
	})
	return results, err
}

// SPARQLSelect executes a simplified SPARQL SELECT.
func (s *Store) SPARQLSelect(query string) ([]map[string]Term, error) {
	patterns, variables, err := parseSimplifiedSPARQL(query)
	if err != nil {
		return nil, err
	}
	if len(patterns) == 0 {
		return nil, nil
	}

	bindings := []map[string]Term{{}}

	for _, p := range patterns {
		var next []map[string]Term
		for _, b := range bindings {
			// Fill in bound variables into pattern.
			resolved := p
			if resolved.Subject == nil {
				if v, ok := b["?s"]; ok {
					resolved.Subject = &v
				}
			}
			if resolved.Predicate == nil {
				if v, ok := b["?p"]; ok {
					resolved.Predicate = &v
				}
			}
			if resolved.Object == nil {
				if v, ok := b["?o"]; ok {
					resolved.Object = &v
				}
			}

			triples, err := s.Query(resolved)
			if err != nil {
				continue
			}
			for _, t := range triples {
				newBinding := copyMap(b)
				if p.Subject == nil {
					newBinding["?s"] = t.Subject
				}
				if p.Predicate == nil {
					newBinding["?p"] = t.Predicate
				}
				if p.Object == nil {
					newBinding["?o"] = t.Object
				}
				next = append(next, newBinding)
			}
		}
		bindings = next
	}

	// Project only requested variables.
	if len(variables) > 0 {
		for i, b := range bindings {
			proj := make(map[string]Term)
			for _, v := range variables {
				if val, ok := b[v]; ok {
					proj[v] = val
				}
			}
			bindings[i] = proj
		}
	}

	return bindings, nil
}

// SPARQLAsk executes a simplified SPARQL ASK query.
func (s *Store) SPARQLAsk(query string) (bool, error) {
	results, err := s.SPARQLSelect(query)
	if err != nil {
		return false, err
	}
	return len(results) > 0, nil
}

// RegisterNamespace adds a prefix-to-URI mapping.
func (s *Store) RegisterNamespace(prefix, uri string) {
	s.namespaces[prefix] = uri
}

// ExpandPrefix expands a prefixed name to full IRI.
func (s *Store) ExpandPrefix(prefixed string) string {
	parts := strings.SplitN(prefixed, ":", 2)
	if len(parts) != 2 {
		return prefixed
	}
	if uri, ok := s.namespaces[parts[0]]; ok {
		return uri + parts[1]
	}
	return prefixed
}

// Count returns the number of triples.
func (s *Store) Count() (int, error) {
	count := 0
	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte(triplePrefix)
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Rewind(); it.Valid(); it.Next() {
			count++
		}
		return nil
	})
	return count, err
}

// ImportNTriples imports N-Triples format data.
func (s *Store) ImportNTriples(data string) (int, error) {
	lines := strings.Split(data, "\n")
	count := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		t, err := parseNTriplesLine(line)
		if err != nil {
			return count, fmt.Errorf("parse line: %w", err)
		}
		if err := s.UpsertTriple(t); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

// ExportNTriples exports all triples in N-Triples format.
func (s *Store) ExportNTriples() (string, error) {
	triples, err := s.Query(TriplePattern{})
	if err != nil {
		return "", err
	}
	var lines []string
	for _, t := range triples {
		lines = append(lines, formatNTriples(t))
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n"), nil
}

func (s *Store) scanIndex(txn *badger.Txn, prefix string, pattern TriplePattern, results *[]Triple) error {
	opts := badger.DefaultIteratorOptions
	opts.Prefix = []byte(prefix)
	it := txn.NewIterator(opts)
	defer it.Close()

	for it.Rewind(); it.Valid(); it.Next() {
		key := it.Item().Key()
		id := string(key[len(prefix):])

		item, err := txn.Get([]byte(triplePrefix + id))
		if err != nil {
			continue
		}
		err = item.Value(func(val []byte) error {
			var t Triple
			if err := json.Unmarshal(val, &t); err != nil {
				return err
			}
			if matchesPattern(t, pattern) {
				*results = append(*results, t)
			}
			return nil
		})
		if err != nil {
			return err
		}
		if pattern.Limit > 0 && len(*results) >= pattern.Limit {
			break
		}
	}
	return nil
}

func matchesPattern(t Triple, p TriplePattern) bool {
	if p.Subject != nil && !termsEqual(t.Subject, *p.Subject) {
		return false
	}
	if p.Predicate != nil && !termsEqual(t.Predicate, *p.Predicate) {
		return false
	}
	if p.Object != nil && !termsEqual(t.Object, *p.Object) {
		return false
	}
	if p.Graph != "" && t.Graph != p.Graph {
		return false
	}
	if p.Inferred != nil && t.Inferred != *p.Inferred {
		return false
	}
	return true
}

func resolvePattern(p TriplePattern, bindings map[string]Term) TriplePattern {
	resolved := p
	if p.Subject != nil && p.Subject.Kind == "" {
		if b, ok := bindings["?s"]; ok {
			resolved.Subject = &Term{Kind: b.Kind, Value: b.Value}
		}
	}
	if p.Object != nil && p.Object.Kind == "" {
		if b, ok := bindings["?o"]; ok {
			resolved.Object = &Term{Kind: b.Kind, Value: b.Value}
		}
	}
	return resolved
}

func termsEqual(a, b Term) bool {
	if a.Kind != b.Kind {
		return false
	}
	return a.Value == b.Value
}

func termKey(t Term) string {
	if t.Language != "" {
		return t.Value + "@" + t.Language
	}
	return t.Value
}

func hashTriple(t *Triple) string {
	h := sha256.Sum256([]byte(t.Subject.Value + "|" + t.Predicate.Value + "|" + t.Object.Value))
	return hex.EncodeToString(h[:8])
}

func copyMap(m map[string]Term) map[string]Term {
	out := make(map[string]Term)
	for k, v := range m {
		out[k] = v
	}
	return out
}
