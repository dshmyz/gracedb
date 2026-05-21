package gracedb

import (
	"github.com/dshmyz/gracedb/pkg/rdf"
)

// Ontology provides a user-friendly API over RDF/RDFS/SHACL.
type Ontology struct {
	store     *rdf.Store
	inferencer *rdf.RDFSInferencer
	validator *rdf.SHACLValidator
}

// Ontology returns the ontology manager for RDF/RDFS/SHACL operations.
func (db *DB) Ontology() *Ontology {
	return &Ontology{
		store:      db.rdf_,
		inferencer: rdf.NewRDFSInferencer(db.rdf_),
		validator:  rdf.NewSHACLValidator(db.rdf_),
	}
}

// DefineClass defines a class with optional parent (subClassOf).
func (o *Ontology) DefineClass(classIRI, parentIRI string) error {
	if err := o.store.UpsertTriple(&rdf.Triple{
		Subject:   rdf.NewIRI(classIRI),
		Predicate: rdf.NewIRI("http://www.w3.org/1999/02/22-rdf-syntax-ns#type"),
		Object:    rdf.NewIRI("http://www.w3.org/2000/01/rdf-schema#Class"),
	}); err != nil {
		return err
	}
	if parentIRI != "" {
		return o.store.UpsertTriple(&rdf.Triple{
			Subject:   rdf.NewIRI(classIRI),
			Predicate: rdf.NewIRI("http://www.w3.org/2000/01/rdf-schema#subClassOf"),
			Object:    rdf.NewIRI(parentIRI),
		})
	}
	return nil
}

// DefineProperty defines a property with optional domain and range.
func (o *Ontology) DefineProperty(propIRI, domainIRI, rangeIRI string) error {
	if err := o.store.UpsertTriple(&rdf.Triple{
		Subject:   rdf.NewIRI(propIRI),
		Predicate: rdf.NewIRI("http://www.w3.org/1999/02/22-rdf-syntax-ns#type"),
		Object:    rdf.NewIRI("http://www.w3.org/1999/02/22-rdf-syntax-ns#Property"),
	}); err != nil {
		return err
	}
	if domainIRI != "" {
		if err := o.store.UpsertTriple(&rdf.Triple{
			Subject:   rdf.NewIRI(propIRI),
			Predicate: rdf.NewIRI("http://www.w3.org/2000/01/rdf-schema#domain"),
			Object:    rdf.NewIRI(domainIRI),
		}); err != nil {
			return err
		}
	}
	if rangeIRI != "" {
		if err := o.store.UpsertTriple(&rdf.Triple{
			Subject:   rdf.NewIRI(propIRI),
			Predicate: rdf.NewIRI("http://www.w3.org/2000/01/rdf-schema#range"),
			Object:    rdf.NewIRI(rangeIRI),
		}); err != nil {
			return err
		}
	}
	return nil
}

// DefineSubProperty defines a subPropertyOf relationship.
func (o *Ontology) DefineSubProperty(childIRI, parentIRI string) error {
	return o.store.UpsertTriple(&rdf.Triple{
		Subject:   rdf.NewIRI(childIRI),
		Predicate: rdf.NewIRI("http://www.w3.org/2000/01/rdf-schema#subPropertyOf"),
		Object:    rdf.NewIRI(parentIRI),
	})
}

// AddType asserts that a resource is an instance of a class.
func (o *Ontology) AddType(resourceIRI, classIRI string) error {
	return o.store.UpsertTriple(&rdf.Triple{
		Subject:   rdf.NewIRI(resourceIRI),
		Predicate: rdf.NewIRI("http://www.w3.org/1999/02/22-rdf-syntax-ns#type"),
		Object:    rdf.NewIRI(classIRI),
	})
}

// AddFact adds a simple triple (resource, predicate, value).
func (o *Ontology) AddFact(resourceIRI, predicateIRI, value string) error {
	return o.store.UpsertTriple(&rdf.Triple{
		Subject:   rdf.NewIRI(resourceIRI),
		Predicate: rdf.NewIRI(predicateIRI),
		Object:    rdf.NewLiteral(value),
	})
}

// AddTypedFact adds a triple with a typed literal value.
func (o *Ontology) AddTypedFact(resourceIRI, predicateIRI, value string, datatype string) error {
	return o.store.UpsertTriple(&rdf.Triple{
		Subject:   rdf.NewIRI(resourceIRI),
		Predicate: rdf.NewIRI(predicateIRI),
		Object:    rdf.NewTypedLiteral(value, datatype),
	})
}

// AddRelation adds an object property (resource → resource).
func (o *Ontology) AddRelation(subjectIRI, predicateIRI, objectIRI string) error {
	return o.store.UpsertTriple(&rdf.Triple{
		Subject:   rdf.NewIRI(subjectIRI),
		Predicate: rdf.NewIRI(predicateIRI),
		Object:    rdf.NewIRI(objectIRI),
	})
}

// Infer runs RDFS inference and materializes new triples.
// Returns the number of new inferred triples.
func (o *Ontology) Infer() (int, error) {
	return o.inferencer.Infer()
}

// ClearInferred removes all previously inferred triples.
func (o *Ontology) ClearInferred() error {
	return o.inferencer.ClearInferred()
}

// DefineShape defines a SHACL node shape with property constraints.
func (o *Ontology) DefineShape(shapeIRI, targetClassIRI string, constraints []SHACLConstraint) error {
	// Shape type.
	if err := o.store.UpsertTriple(&rdf.Triple{
		Subject:   rdf.NewIRI(shapeIRI),
		Predicate: rdf.NewIRI("http://www.w3.org/1999/02/22-rdf-syntax-ns#type"),
		Object:    rdf.NewIRI("http://www.w3.org/ns/shacl#NodeShape"),
	}); err != nil {
		return err
	}
	// Target class.
	if targetClassIRI != "" {
		if err := o.store.UpsertTriple(&rdf.Triple{
			Subject:   rdf.NewIRI(shapeIRI),
			Predicate: rdf.NewIRI("http://www.w3.org/ns/shacl#targetClass"),
			Object:    rdf.NewIRI(targetClassIRI),
		}); err != nil {
			return err
		}
	}
	// Property constraints.
	for _, c := range constraints {
		propNode := shapeIRI + "_prop_" + c.Path
		if err := o.store.UpsertTriple(&rdf.Triple{
			Subject:   rdf.NewIRI(shapeIRI),
			Predicate: rdf.NewIRI("http://www.w3.org/ns/shacl#property"),
			Object:    rdf.NewIRI(propNode),
		}); err != nil {
			return err
		}
		if err := o.store.UpsertTriple(&rdf.Triple{
			Subject:   rdf.NewIRI(propNode),
			Predicate: rdf.NewIRI("http://www.w3.org/ns/shacl#path"),
			Object:    rdf.NewIRI(c.Path),
		}); err != nil {
			return err
		}
		if c.MinCount > 0 {
			if err := o.store.UpsertTriple(&rdf.Triple{
				Subject:   rdf.NewIRI(propNode),
				Predicate: rdf.NewIRI("http://www.w3.org/ns/shacl#minCount"),
				Object:    rdf.NewLiteral(c.MinCountStr()),
			}); err != nil {
				return err
			}
		}
		if c.MaxCount > 0 {
			if err := o.store.UpsertTriple(&rdf.Triple{
				Subject:   rdf.NewIRI(propNode),
				Predicate: rdf.NewIRI("http://www.w3.org/ns/shacl#maxCount"),
				Object:    rdf.NewLiteral(c.MaxCountStr()),
			}); err != nil {
				return err
			}
		}
		if c.Datatype != "" {
			if err := o.store.UpsertTriple(&rdf.Triple{
				Subject:   rdf.NewIRI(propNode),
				Predicate: rdf.NewIRI("http://www.w3.org/ns/shacl#datatype"),
				Object:    rdf.NewIRI(c.Datatype),
			}); err != nil {
				return err
			}
		}
		if c.Pattern != "" {
			if err := o.store.UpsertTriple(&rdf.Triple{
				Subject:   rdf.NewIRI(propNode),
				Predicate: rdf.NewIRI("http://www.w3.org/ns/shacl#pattern"),
				Object:    rdf.NewLiteral(c.Pattern),
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

// Validate runs SHACL validation against all defined shapes.
func (o *Ontology) Validate() (*rdf.ValidationReport, error) {
	return o.validator.Validate()
}

// Count returns the total number of triples.
func (o *Ontology) Count() (int, error) {
	return o.store.Count()
}

// Query runs a pattern match query. Nil fields are wildcards.
func (o *Ontology) Query(subject, predicate, object string) ([]rdf.Triple, error) {
	pattern := rdf.TriplePattern{}
	if subject != "" {
		pattern.Subject = &rdf.Term{Kind: rdf.TermIRI, Value: subject}
	}
	if predicate != "" {
		pattern.Predicate = &rdf.Term{Kind: rdf.TermIRI, Value: predicate}
	}
	if object != "" {
		pattern.Object = &rdf.Term{Kind: rdf.TermIRI, Value: object}
	}
	return o.store.Query(pattern)
}

// SPARQLSelect runs a simplified SPARQL SELECT query.
func (o *Ontology) SPARQLSelect(query string) ([]map[string]rdf.Term, error) {
	return o.store.SPARQLSelect(query)
}

// SPARQLAsk runs a simplified SPARQL ASK query.
func (o *Ontology) SPARQLAsk(query string) (bool, error) {
	return o.store.SPARQLAsk(query)
}

// SHACLConstraint defines a property constraint for a SHACL shape.
type SHACLConstraint struct {
	Path     string
	MinCount int
	MaxCount int
	Datatype string
	Pattern  string
}

func (c SHACLConstraint) MinCountStr() string {
	if c.MinCount == 0 {
		return ""
	}
	return itoa(c.MinCount)
}

func (c SHACLConstraint) MaxCountStr() string {
	if c.MaxCount == 0 {
		return ""
	}
	return itoa(c.MaxCount)
}

func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
