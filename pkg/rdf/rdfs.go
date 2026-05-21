package rdf

// RDFS constants.
const (
	RDFType         = "http://www.w3.org/1999/02/22-rdf-syntax-ns#type"
	RDFSClass       = "http://www.w3.org/2000/01/rdf-schema#Class"
	RDFSSubClassOf  = "http://www.w3.org/2000/01/rdf-schema#subClassOf"
	RDFSLabel       = "http://www.w3.org/2000/01/rdf-schema#label"
	RDFSSubPropOf   = "http://www.w3.org/2000/01/rdf-schema#subPropertyOf"
	RDFSDomain      = "http://www.w3.org/2000/01/rdf-schema#domain"
	RDFSRange       = "http://www.w3.org/2000/01/rdf-schema#range"
)

// RDFSInferencer performs RDFS-lite materialized inference.
type RDFSInferencer struct {
	store *Store
}

// NewRDFSInferencer creates an RDFS inferencer.
func NewRDFSInferencer(store *Store) *RDFSInferencer {
	return &RDFSInferencer{store: store}
}

// Infer applies RDFS inference rules and materializes new triples.
// Returns the number of new inferred triples.
func (inf *RDFSInferencer) Infer() (int, error) {
	count := 0

	// Rule 1: subClassOf transitivity
	// If A subClassOf B and B subClassOf C, infer A subClassOf C.
	subClassRules, err := inf.inferSubClassTransitivity()
	if err != nil {
		return count, err
	}
	count += subClassRules

	// Rule 2: rdf:type propagation
	// If X type A and A subClassOf B, infer X type B.
	typeRules, err := inf.inferTypePropagation()
	if err != nil {
		return count, err
	}
	count += typeRules

	// Rule 3: subPropertyOf transitivity
	// If P subPropertyOf Q and Q subPropertyOf R, infer P subPropertyOf R.
	subPropRules, err := inf.inferSubPropertyTransitivity()
	if err != nil {
		return count, err
	}
	count += subPropRules

	// Rule 4: subPropertyOf type propagation
	// If X P Y and P subPropertyOf Q, infer X Q Y.
	propTypeRules, err := inf.inferPropertyTypePropagation()
	if err != nil {
		return count, err
	}
	count += propTypeRules

	// Rule 5: domain inference
	// If X P Y and P domain D, infer X type D.
	domainRules, err := inf.inferDomain()
	if err != nil {
		return count, err
	}
	count += domainRules

	// Rule 6: range inference
	// If X P Y and P range R, infer Y type R.
	rangeRules, err := inf.inferRange()
	if err != nil {
		return count, err
	}
	count += rangeRules

	return count, nil
}

// ClearInferred removes all previously inferred triples.
func (inf *RDFSInferencer) ClearInferred() error {
	triples, err := inf.store.Query(TriplePattern{Inferred: ptrBool(true)})
	if err != nil {
		return err
	}
	for _, t := range triples {
		if err := inf.store.DeleteTriple(t.ID); err != nil {
			return err
		}
	}
	return nil
}

func (inf *RDFSInferencer) inferSubClassTransitivity() (int, error) {
	// Get all subClassOf triples.
	subClassOf := TriplePattern{Predicate: &Term{Kind: TermIRI, Value: RDFSSubClassOf}}
	subClassTriples, err := inf.store.Query(subClassOf)
	if err != nil {
		return 0, err
	}

	// Build adjacency: child -> parent.
	type edge struct {
		child  string
		parent string
	}
	var edges []edge
	for _, t := range subClassTriples {
		edges = append(edges, edge{t.Subject.Value, t.Object.Value})
	}

	// Find transitive closures.
	count := 0
	for _, e1 := range edges {
		for _, e2 := range edges {
			if e1.parent == e2.child && e1.child != e2.parent {
				// e1.child subClassOf e1.parent (=e2.child) subClassOf e2.parent
				// => infer e1.child subClassOf e2.parent
				newTriple := &Triple{
					Subject:  NewIRI(e1.child),
					Predicate: NewIRI(RDFSSubClassOf),
					Object:   NewIRI(e2.parent),
					Inferred: true,
				}
				if !inf.exists(newTriple) {
					if err := inf.store.UpsertTriple(newTriple); err != nil {
						return count, err
					}
					count++
				}
			}
		}
	}
	return count, nil
}

func (inf *RDFSInferencer) inferTypePropagation() (int, error) {
	// Get all type triples.
	typePattern := TriplePattern{Predicate: &Term{Kind: TermIRI, Value: RDFType}}
	typeTriples, err := inf.store.Query(typePattern)
	if err != nil {
		return 0, err
	}

	// Get all subClassOf triples.
	subClassOfPattern := TriplePattern{Predicate: &Term{Kind: TermIRI, Value: RDFSSubClassOf}}
	subClassTriples, err := inf.store.Query(subClassOfPattern)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, tt := range typeTriples {
		for _, st := range subClassTriples {
			if tt.Object.Value == st.Subject.Value && tt.Subject.Value != st.Object.Value {
				// X type A, A subClassOf B => X type B
				newTriple := &Triple{
					Subject:  tt.Subject,
					Predicate: NewIRI(RDFType),
					Object:   st.Object,
					Inferred: true,
				}
				if !inf.exists(newTriple) {
					if err := inf.store.UpsertTriple(newTriple); err != nil {
						return count, err
					}
					count++
				}
			}
		}
	}
	return count, nil
}

func (inf *RDFSInferencer) inferSubPropertyTransitivity() (int, error) {
	subPropPattern := TriplePattern{Predicate: &Term{Kind: TermIRI, Value: RDFSSubPropOf}}
	subPropTriples, err := inf.store.Query(subPropPattern)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, e1 := range subPropTriples {
		for _, e2 := range subPropTriples {
			if e1.Object.Value == e2.Subject.Value && e1.Subject.Value != e2.Object.Value {
				newTriple := &Triple{
					Subject:  e1.Subject,
					Predicate: NewIRI(RDFSSubPropOf),
					Object:   e2.Object,
					Inferred: true,
				}
				if !inf.exists(newTriple) {
					if err := inf.store.UpsertTriple(newTriple); err != nil {
						return count, err
					}
					count++
				}
			}
		}
	}
	return count, nil
}

func (inf *RDFSInferencer) inferPropertyTypePropagation() (int, error) {
	subPropPattern := TriplePattern{Predicate: &Term{Kind: TermIRI, Value: RDFSSubPropOf}}
	subPropTriples, err := inf.store.Query(subPropPattern)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, sp := range subPropTriples {
		// Find all triples using the sub-property.
		pattern := TriplePattern{Predicate: &Term{Kind: TermIRI, Value: sp.Subject.Value}}
		dataTriples, err := inf.store.Query(pattern)
		if err != nil {
			continue
		}
		for _, dt := range dataTriples {
			newTriple := &Triple{
				Subject:  dt.Subject,
				Predicate: sp.Object,
				Object:   dt.Object,
				Inferred: true,
			}
			if !inf.exists(newTriple) {
				if err := inf.store.UpsertTriple(newTriple); err != nil {
					return count, err
				}
				count++
			}
		}
	}
	return count, nil
}

func (inf *RDFSInferencer) inferDomain() (int, error) {
	domainPattern := TriplePattern{Predicate: &Term{Kind: TermIRI, Value: RDFSDomain}}
	domainTriples, err := inf.store.Query(domainPattern)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, dt := range domainTriples {
		// P domain D. Find all X P Y triples.
		pattern := TriplePattern{Predicate: &Term{Kind: TermIRI, Value: dt.Subject.Value}}
		dataTriples, err := inf.store.Query(pattern)
		if err != nil {
			continue
		}
		for _, xt := range dataTriples {
			// Infer: X type D
			newTriple := &Triple{
				Subject:  xt.Subject,
				Predicate: NewIRI(RDFType),
				Object:   dt.Object,
				Inferred: true,
			}
			if !inf.exists(newTriple) {
				if err := inf.store.UpsertTriple(newTriple); err != nil {
					return count, err
				}
				count++
			}
		}
	}
	return count, nil
}

func (inf *RDFSInferencer) inferRange() (int, error) {
	rangePattern := TriplePattern{Predicate: &Term{Kind: TermIRI, Value: RDFSRange}}
	rangeTriples, err := inf.store.Query(rangePattern)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, rt := range rangeTriples {
		pattern := TriplePattern{Predicate: &Term{Kind: TermIRI, Value: rt.Subject.Value}}
		dataTriples, err := inf.store.Query(pattern)
		if err != nil {
			continue
		}
		for _, xt := range dataTriples {
			// Infer: Y type R
			newTriple := &Triple{
				Subject:  xt.Object,
				Predicate: NewIRI(RDFType),
				Object:   rt.Object,
				Inferred: true,
			}
			if !inf.exists(newTriple) {
				if err := inf.store.UpsertTriple(newTriple); err != nil {
					return count, err
				}
				count++
			}
		}
	}
	return count, nil
}

func (inf *RDFSInferencer) exists(t *Triple) bool {
	pattern := TriplePattern{
		Subject:  &t.Subject,
		Predicate: &t.Predicate,
		Object:   &t.Object,
	}
	results, err := inf.store.Query(pattern)
	if err != nil {
		return false
	}
	for _, r := range results {
		if r.Subject.Value == t.Subject.Value &&
			r.Predicate.Value == t.Predicate.Value &&
			r.Object.Value == t.Object.Value {
			return true
		}
	}
	return false
}

func ptrBool(b bool) *bool {
	return &b
}
