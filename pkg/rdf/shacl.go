package rdf

import (
	"fmt"
	"strings"
)

// SHACL constraint types.
const (
	SHACLNodeShape    = "http://www.w3.org/ns/shacl#NodeShape"
	SHACLTargetClass  = "http://www.w3.org/ns/shacl#targetClass"
	SHACLDatatype     = "http://www.w3.org/ns/shacl#datatype"
	SHACLMinCount     = "http://www.w3.org/ns/shacl#minCount"
	SHACLMaxCount     = "http://www.w3.org/ns/shacl#maxCount"
	SHACLPattern      = "http://www.w3.org/ns/shacl#pattern"
	SHACLMinInclusive = "http://www.w3.org/ns/shacl#minInclusive"
	SHACLMaxInclusive = "http://www.w3.org/ns/shacl#maxInclusive"
	SHACLProperty     = "http://www.w3.org/ns/shacl#property"
	SHACLPath         = "http://www.w3.org/ns/shacl#path"
	SHACLMessage      = "http://www.w3.org/ns/shacl#message"
)

// ValidationReport is the output of SHACL validation.
type ValidationReport struct {
	Conforms       bool               `json:"conforms"`
	Violations     []ValidationResult `json:"violations"`
	CheckedShapes  int                `json:"checked_shapes"`
	CheckedNodes   int                `json:"checked_nodes"`
}

// ValidationResult is a single violation.
type ValidationResult struct {
	ShapeID   string `json:"shape_id"`
	FocusNode string `json:"focus_node"`
	Message   string `json:"message"`
	Path      string `json:"path"`
}

// SHACLValidator performs SHACL-lite validation.
type SHACLValidator struct {
	store *Store
}

// NewSHACLValidator creates a SHACL validator.
func NewSHACLValidator(store *Store) *SHACLValidator {
	return &SHACLValidator{store: store}
}

// Validate checks all shapes against their target nodes.
func (v *SHACLValidator) Validate() (*ValidationReport, error) {
	report := &ValidationReport{Conforms: true}

	// Find all node shapes.
	shapePattern := TriplePattern{
		Predicate: &Term{Kind: TermIRI, Value: "http://www.w3.org/1999/02/22-rdf-syntax-ns#type"},
		Object:    &Term{Kind: TermIRI, Value: SHACLNodeShape},
	}
	shapes, err := v.store.Query(shapePattern)
	if err != nil {
		return nil, err
	}
	report.CheckedShapes = len(shapes)

	// Validate each shape.
	for _, shape := range shapes {
		shapeID := shape.Subject.Value
		if err := v.validateShape(shapeID, report); err != nil {
			return nil, fmt.Errorf("validate shape %s: %w", shapeID, err)
		}
	}

	return report, nil
}

func (v *SHACLValidator) validateShape(shapeID string, report *ValidationReport) error {
	// Find target class.
	targetClass, err := v.getObject(shapeID, SHACLTargetClass)
	if err != nil {
		return nil // No target, skip.
	}
	_ = targetClass

	// Find all instances of target class.
	typePattern := TriplePattern{
		Predicate: &Term{Kind: TermIRI, Value: RDFType},
		Object:    &Term{Kind: TermIRI, Value: targetClass},
	}
	instances, err := v.store.Query(typePattern)
	if err != nil {
		return err
	}
	report.CheckedNodes += len(instances)

	// Find property constraints for this shape.
	propPattern := TriplePattern{
		Subject: &Term{Kind: TermIRI, Value: shapeID},
		Predicate: &Term{Kind: TermIRI, Value: SHACLProperty},
	}
	props, err := v.store.Query(propPattern)
	if err != nil {
		return err
	}

	for _, prop := range props {
		propNode := prop.Object.Value
		if err := v.validateProperty(propNode, instances, report); err != nil {
			return err
		}
	}

	// Validate minCount constraint for each property.
	for _, prop := range props {
		propNode := prop.Object.Value
		path, err := v.getObject(propNode, SHACLPath)
		if err != nil {
			continue
		}
		minCount, err := v.getLiteralInt(propNode, SHACLMinCount)
		if err != nil || minCount <= 0 {
			continue
		}
		for _, inst := range instances {
			count, _ := v.countProperty(inst.Subject.Value, path)
			if count < minCount {
				report.Conforms = false
				report.Violations = append(report.Violations, ValidationResult{
					ShapeID:   shapeID,
					FocusNode: inst.Subject.Value,
					Path:      path,
					Message:   fmt.Sprintf("minCount violation: expected at least %d, got %d", minCount, count),
				})
			}
		}
	}

	return nil
}

func (v *SHACLValidator) validateProperty(propNode string, instances []Triple, report *ValidationReport) error {
	// Get the path (property IRI).
	path, err := v.getObject(propNode, SHACLPath)
	if err != nil {
		return nil
	}

	// Check datatype constraint.
	datatype, err := v.getObject(propNode, SHACLDatatype)
	if err == nil {
		for _, inst := range instances {
			values, _ := v.getPropertyValues(inst.Subject.Value, path)
			for _, val := range values {
				if val.Kind != TermLiteral || val.Datatype != datatype {
					report.Conforms = false
					report.Violations = append(report.Violations, ValidationResult{
						ShapeID:   propNode,
						FocusNode: inst.Subject.Value,
						Path:      path,
						Message:   fmt.Sprintf("datatype violation: expected %s", datatype),
					})
				}
			}
		}
	}

	// Check pattern constraint.
	pattern, err := v.getObject(propNode, SHACLPattern)
	if err == nil {
		for _, inst := range instances {
			values, _ := v.getPropertyValues(inst.Subject.Value, path)
			for _, val := range values {
				if val.Kind == TermLiteral && !matchesRegexPattern(val.Value, pattern) {
					report.Conforms = false
					report.Violations = append(report.Violations, ValidationResult{
						ShapeID:   propNode,
						FocusNode: inst.Subject.Value,
						Path:      path,
						Message:   fmt.Sprintf("pattern violation: value %q does not match %s", val.Value, pattern),
					})
				}
			}
		}
	}

	return nil
}

func (v *SHACLValidator) getObject(subject, predicate string) (string, error) {
	results, err := v.store.Query(TriplePattern{
		Subject:   &Term{Kind: TermIRI, Value: subject},
		Predicate: &Term{Kind: TermIRI, Value: predicate},
	})
	if err != nil || len(results) == 0 {
		return "", fmt.Errorf("not found")
	}
	return results[0].Object.Value, nil
}

func (v *SHACLValidator) getLiteralInt(subject, predicate string) (int, error) {
	results, err := v.store.Query(TriplePattern{
		Subject:   &Term{Kind: TermIRI, Value: subject},
		Predicate: &Term{Kind: TermIRI, Value: predicate},
	})
	if err != nil || len(results) == 0 {
		return 0, fmt.Errorf("not found")
	}
	val := results[0].Object.Value
	var n int
	fmt.Sscanf(val, "%d", &n)
	return n, nil
}

func (v *SHACLValidator) getPropertyValues(subject, predicate string) ([]Term, error) {
	results, err := v.store.Query(TriplePattern{
		Subject:   &Term{Kind: TermIRI, Value: subject},
		Predicate: &Term{Kind: TermIRI, Value: predicate},
	})
	if err != nil {
		return nil, err
	}
	var values []Term
	for _, r := range results {
		values = append(values, r.Object)
	}
	return values, nil
}

func (v *SHACLValidator) countProperty(subject, predicate string) (int, error) {
	results, err := v.getPropertyValues(subject, predicate)
	return len(results), err
}

func matchesRegexPattern(value, pattern string) bool {
	// Simple substring match as regex-lite.
	return strings.Contains(value, pattern)
}
