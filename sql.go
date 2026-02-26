package entigo

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// cte represents a Common Table Expression (WITH clause) used in SQL queries.
type cte struct {
	name  string
	query string
	args  []any
}

// SQLBuilder provides a fluent API for constructing complex SQL queries with
// support for CTEs, DISTINCT ON, subqueries, and filter application.
// Arguments are stored per-section and concatenated in SQL clause order by Build().
type SQLBuilder struct {
	withClauses []cte
	selectPart  string
	selectArgs  []any
	fromPart    string
	fromArgs    []any
	wherePart   []string
	whereArgs   []any
	groupBy     []string
	orderBy     []string
	distinctOn  []string
}

// NewSQLBuilder creates a new SQLBuilder instance with pre-allocated slices.
func NewSQLBuilder() *SQLBuilder {
	return &SQLBuilder{
		withClauses: make([]cte, 0),
		selectArgs:  make([]any, 0),
		fromArgs:    make([]any, 0),
		wherePart:   make([]string, 0),
		whereArgs:   make([]any, 0),
		groupBy:     make([]string, 0),
		orderBy:     make([]string, 0),
		distinctOn:  make([]string, 0),
	}
}

// With adds a CTE (Common Table Expression) with a raw query string and
// optional arguments.
func (b *SQLBuilder) With(name string, query string, args ...any) *SQLBuilder {
	b.withClauses = append(b.withClauses, cte{
		name:  name,
		query: query,
		args:  args,
	})
	return b
}

// WithBuilder adds a CTE whose query is built from another SQLBuilder.
func (b *SQLBuilder) WithBuilder(name string, builder *SQLBuilder) *SQLBuilder {
	query, args := builder.Build()
	b.withClauses = append(b.withClauses, cte{
		name:  name,
		query: query,
		args:  args,
	})
	return b
}

// DistinctOn adds DISTINCT ON columns to the query (PostgreSQL-specific).
func (b *SQLBuilder) DistinctOn(columns ...string) *SQLBuilder {
	b.distinctOn = append(b.distinctOn, columns...)
	return b
}

// Select sets the SELECT part of the query with optional arguments for
// parameterized expressions.
func (b *SQLBuilder) Select(selectPart string, args ...any) *SQLBuilder {
	b.selectPart = selectPart
	b.selectArgs = append(b.selectArgs, args...)
	return b
}

// From sets the FROM part of the query with optional arguments for subqueries
// or parameterized table expressions.
func (b *SQLBuilder) From(fromPart string, args ...any) *SQLBuilder {
	b.fromPart = fromPart
	b.fromArgs = append(b.fromArgs, args...)
	return b
}

// Where adds a WHERE condition with arguments. Multiple calls produce
// conditions joined with AND.
func (b *SQLBuilder) Where(condition string, args ...any) *SQLBuilder {
	b.wherePart = append(b.wherePart, condition)
	b.whereArgs = append(b.whereArgs, args...)
	return b
}

// GroupBy adds columns to the GROUP BY clause.
func (b *SQLBuilder) GroupBy(columns ...string) *SQLBuilder {
	b.groupBy = append(b.groupBy, columns...)
	return b
}

// OrderBy adds columns to the ORDER BY clause.
func (b *SQLBuilder) OrderBy(columns ...string) *SQLBuilder {
	b.orderBy = append(b.orderBy, columns...)
	return b
}

// Build generates the final SQL query string and its arguments.
// Arguments are collected in SQL clause order: WITH, SELECT, FROM, WHERE.
func (b *SQLBuilder) Build() (string, []any) {
	var query strings.Builder
	var args []any

	// Build WITH clause if any
	if len(b.withClauses) > 0 {
		query.WriteString("WITH ")
		for i, c := range b.withClauses {
			if i > 0 {
				query.WriteString(", ")
			}
			query.WriteString(c.name)
			query.WriteString(" AS (")
			query.WriteString(c.query)
			query.WriteString(")")
			args = append(args, c.args...)
		}
		query.WriteString(" ")
	}

	// Build SELECT
	query.WriteString("SELECT ")

	// Add DISTINCT ON if specified
	if len(b.distinctOn) > 0 {
		query.WriteString("DISTINCT ON (")
		query.WriteString(strings.Join(b.distinctOn, ", "))
		query.WriteString(") ")
	}

	query.WriteString(b.selectPart)
	args = append(args, b.selectArgs...)

	// Build FROM
	query.WriteString(" FROM ")
	query.WriteString(b.fromPart)
	args = append(args, b.fromArgs...)

	// Build WHERE
	if len(b.wherePart) > 0 {
		query.WriteString(" WHERE ")
		query.WriteString(strings.Join(b.wherePart, " AND "))
	}
	args = append(args, b.whereArgs...)

	// Build GROUP BY
	if len(b.groupBy) > 0 {
		query.WriteString(" GROUP BY ")
		query.WriteString(strings.Join(b.groupBy, ", "))
	}

	// Build ORDER BY
	if len(b.orderBy) > 0 {
		query.WriteString(" ORDER BY ")
		query.WriteString(strings.Join(b.orderBy, ", "))
	}

	return query.String(), args
}

// ApplyFilter applies a filter map to the query builder. Each entry is
// processed through processFilter to support operator prefixes.
func (b *SQLBuilder) ApplyFilter(filters map[string]any) *SQLBuilder {
	for field, value := range filters {
		if !IsValidColumnName(field) {
			continue
		}
		fp := processFilter(field, value)
		if fp.Condition != "" {
			b.Where(fp.Condition, fp.Args...)
		}
	}
	return b
}

// ExecutePaginatedQuery is a generic paginated query executor. It runs a count
// query to determine the total number of matching records, then fetches the
// requested page using LIMIT/OFFSET.
func ExecutePaginatedQuery[T any](
	ctx context.Context,
	db *gorm.DB,
	qb *SQLBuilder,
	offset, size int,
) (int64, []T, error) {
	var result []T

	// Get base query and args
	query, args := qb.Build()

	// Count total records
	var total int64
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM (%s) as total", query)
	if err := db.Raw(countQuery, args...).Count(&total).Error; err != nil {
		return 0, nil, fmt.Errorf("count query error: %w", err)
	}

	// Get paginated results
	if err := db.Raw(query+" LIMIT ? OFFSET ?", append(args, size, offset)...).Scan(&result).Error; err != nil {
		return 0, nil, fmt.Errorf("fetch data error: %w", err)
	}

	return total, result, nil
}
