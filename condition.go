package entigo

import (
	"fmt"
	"strings"
)

// Operator represents logical operators used to combine conditions.
type Operator string

const (
	AND Operator = "AND"
	OR  Operator = "OR"
)

// Condition represents a single SQL condition or a group of nested conditions.
type Condition struct {
	operator   Operator
	query      string
	args       []any
	conditions []*Condition
	parent     *Condition
}

// ConditionBuilder provides a fluent API for constructing complex SQL WHERE clauses
// with support for AND/OR operators, nested groups, and conditional inclusion.
type ConditionBuilder struct {
	root    *Condition
	current *Condition
}

// NewConditionBuilder creates a new ConditionBuilder with an AND root group.
func NewConditionBuilder() *ConditionBuilder {
	root := &Condition{
		operator: AND,
	}
	return &ConditionBuilder{
		root:    root,
		current: root,
	}
}

// Condition adds one or more conditions without specifying an operator.
// When multiple pairs of (query, args) are provided, they are each added as AND conditions.
// A trailing single argument is also added as an AND condition.
func (cb *ConditionBuilder) Condition(args ...any) *ConditionBuilder {
	if len(args) == 0 {
		return cb
	}
	for len(args) >= 2 {
		query, queryArgs := extractQueryAndArgs(args...)
		condition := &Condition{
			operator: AND,
			query:    query,
			args:     queryArgs,
			parent:   cb.current,
		}
		cb.current.conditions = append(cb.current.conditions, condition)
		args = args[2:]
	}
	return cb.And(args...)
}

// And adds a condition joined with the AND operator.
func (cb *ConditionBuilder) And(args ...any) *ConditionBuilder {
	if len(args) == 0 {
		return cb
	}
	query, queryArgs := extractQueryAndArgs(args...)
	condition := &Condition{
		operator: AND,
		query:    query,
		args:     queryArgs,
		parent:   cb.current,
	}
	cb.current.conditions = append(cb.current.conditions, condition)
	return cb
}

// Or adds a condition joined with the OR operator.
func (cb *ConditionBuilder) Or(args ...any) *ConditionBuilder {
	if len(args) == 0 {
		return cb
	}
	query, queryArgs := extractQueryAndArgs(args...)
	condition := &Condition{
		operator: OR,
		query:    query,
		args:     queryArgs,
		parent:   cb.current,
	}
	cb.current.conditions = append(cb.current.conditions, condition)
	return cb
}

// extractQueryAndArgs separates the first argument as the SQL query string
// and returns the remaining arguments as query parameters.
func extractQueryAndArgs(args ...any) (string, []any) {
	if len(args) == 0 {
		return "", nil
	}
	query, ok := args[0].(string)
	if !ok {
		return "", nil
	}
	return query, args[1:]
}

// GroupStart begins a new nested AND group. All subsequent conditions are added
// to this group until GroupEnd is called.
func (cb *ConditionBuilder) GroupStart() *ConditionBuilder {
	group := &Condition{
		operator: AND,
		parent:   cb.current,
	}
	cb.current.conditions = append(cb.current.conditions, group)
	cb.current = group
	return cb
}

// OrGroupStart begins a new nested OR group. All subsequent conditions are added
// to this group until GroupEnd is called.
func (cb *ConditionBuilder) OrGroupStart() *ConditionBuilder {
	group := &Condition{
		operator: OR,
		parent:   cb.current,
	}
	cb.current.conditions = append(cb.current.conditions, group)
	cb.current = group
	return cb
}

// GroupEnd closes the current nested group and returns to the parent scope.
func (cb *ConditionBuilder) GroupEnd() *ConditionBuilder {
	if cb.current.parent != nil {
		cb.current = cb.current.parent
	}
	return cb
}

// ConditionIf adds a condition only when the check function returns true.
func (cb *ConditionBuilder) ConditionIf(check func() bool, args ...any) *ConditionBuilder {
	if check() {
		return cb.Condition(args...)
	}
	return cb
}

// AndIf adds an AND condition only when the check function returns true.
func (cb *ConditionBuilder) AndIf(check func() bool, args ...any) *ConditionBuilder {
	if check() {
		return cb.And(args...)
	}
	return cb
}

// OrIf adds an OR condition only when the check function returns true.
func (cb *ConditionBuilder) OrIf(check func() bool, args ...any) *ConditionBuilder {
	if check() {
		return cb.Or(args...)
	}
	return cb
}

// Build generates the final SQL WHERE clause and its arguments.
// Returns an empty string and nil args if no conditions have been added.
func (cb *ConditionBuilder) Build() (string, []any) {
	if cb.root == nil || (cb.root.query == "" && len(cb.root.conditions) == 0) {
		return "", nil
	}

	var args []any
	query := buildCondition(cb.root, &args)
	return query, args
}

// HasConditions returns true if the builder contains at least one condition.
func (cb *ConditionBuilder) HasConditions() bool {
	return cb.root != nil && (cb.root.query != "" || len(cb.root.conditions) > 0)
}

// IsEmpty returns true if the builder has no conditions.
func (cb *ConditionBuilder) IsEmpty() bool {
	return !cb.HasConditions()
}

// buildCondition recursively builds the SQL string for a condition tree.
func buildCondition(c *Condition, args *[]any) string {
	if c == nil {
		return ""
	}

	var parts []string
	if c.query != "" {
		parts = append(parts, c.query)
		*args = append(*args, c.args...)
	}

	for _, cond := range c.conditions {
		subQuery := buildCondition(cond, args)
		if subQuery != "" {
			if len(c.conditions) > 1 || c.query != "" {
				subQuery = fmt.Sprintf("(%s)", subQuery)
			}
			if len(parts) > 0 {
				parts = append(parts, string(cond.operator), subQuery)
			} else {
				parts = append(parts, subQuery)
			}
		}
	}

	return strings.Join(parts, " ")
}

// NotEmpty returns a check function that returns true when the string value is non-empty.
func NotEmpty(value string) func() bool {
	return func() bool {
		return value != ""
	}
}

// NotZero returns a check function that returns true when the int value is non-zero.
func NotZero(value int) func() bool {
	return func() bool {
		return value != 0
	}
}

// NotNil returns a check function that returns true when the value is not nil.
func NotNil(value any) func() bool {
	return func() bool {
		return value != nil
	}
}
