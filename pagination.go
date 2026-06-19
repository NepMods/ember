package ember

import (
	"context"
	"math"
	"reflect"
)

// Paginator holds pagination metadata and the result items.
type Paginator struct {
	CurrentPage int         `json:"current_page"`
	LastPage    int         `json:"last_page"`
	PerPage     int         `json:"per_page"`
	Total       int64       `json:"total"`
	From        int         `json:"from"`
	To          int         `json:"to"`
	Items       interface{} `json:"items"`
}

// Paginate paginates the builder results into dest.
func (b *Builder) Paginate(ctx context.Context, dest interface{}, page, perPage int) (*Paginator, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 15
	}

	total, err := b.Count(ctx)
	if err != nil {
		return nil, err
	}

	lastPage := int(math.Ceil(float64(total) / float64(perPage)))
	if lastPage < 1 {
		lastPage = 1
	}

	clone := b.clone()
	clone.ForPage(page, perPage)
	if err := clone.Get(ctx, dest); err != nil {
		return nil, err
	}

	from := (page-1)*perPage + 1
	to := page * perPage
	if int64(to) > total {
		to = int(total)
	}
	if total == 0 {
		from = 0
		to = 0
	}

	return &Paginator{
		CurrentPage: page,
		LastPage:    lastPage,
		PerPage:     perPage,
		Total:       total,
		From:        from,
		To:          to,
		Items:       dest,
	}, nil
}

// Paginate paginates the model query results into dest.
func (m *ModelDB) Paginate(ctx context.Context, dest interface{}, page, perPage int, fn ...func(*Builder)) (*Paginator, error) {
	schema, err := parseSchemaFromSlice(dest)
	if err != nil {
		return nil, err
	}

	b := m.builder(schema.TableName)
	if schema.HasSoftDelete {
		b = b.WhereNull("deleted_at")
	}

	if m.db != nil {
		scopes := m.db.ScopeRegistry().Get(schema.GoType)
		b = ApplyScopes(b, scopes...)
	}

	for _, f := range fn {
		f(b)
	}

	paginator, err := b.Paginate(ctx, dest, page, perPage)
	if err != nil {
		return nil, err
	}

	if len(m.relationLoads) > 0 {
		sliceVal := reflect.ValueOf(dest).Elem()
		if err := m.EagerLoadSlice(ctx, sliceVal, schema, m.relationLoads); err != nil {
			return nil, err
		}
	}

	return paginator, nil
}
