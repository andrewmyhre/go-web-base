package db

import (
	"context"
	"fmt"

	"github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/persist"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	sqlc "github.com/andrewmyhre/go-web-base/core/db/sqlc"
)

// CasbinAdapter is the API's own Casbin adapter: it stores policy rules
// in the casbin_rule table created by the goose migrations and reads and
// writes them through pgx and the sqlc-generated queries. The maintained
// Casbin Postgres adapter is GORM-based and creates its own table outside
// the migrations, so it is not used.
//
// It implements casbin's persist.Adapter, persist.BatchAdapter and
// persist.FilteredAdapter.
type CasbinAdapter struct {
	db       sqlc.DBTX
	filtered bool // true if the most recent load was a filtered one
}

// NewCasbinAdapter returns a CasbinAdapter reading and writing through
// db.
func NewCasbinAdapter(db sqlc.DBTX) *CasbinAdapter {
	return &CasbinAdapter{db: db}
}

var (
	_ persist.Adapter         = (*CasbinAdapter)(nil)
	_ persist.BatchAdapter    = (*CasbinAdapter)(nil)
	_ persist.FilteredAdapter = (*CasbinAdapter)(nil)
)

// maxValues is the number of value columns casbin_rule has.
const maxValues = 6

// valuesFromRule pads or bounds rule to exactly maxValues pgtype.Text
// arguments, the trailing ones NULL.
func valuesFromRule(rule []string) ([]pgtype.Text, error) {
	if len(rule) > maxValues {
		return nil, fmt.Errorf("casbin rule has %d values, casbin_rule holds at most %d", len(rule), maxValues)
	}
	values := make([]pgtype.Text, maxValues)
	for i, v := range rule {
		values[i] = pgtype.Text{String: v, Valid: true}
	}
	return values, nil
}

// ruleFromRow drops the trailing NULLs of a stored rule.
func ruleFromRow(row sqlc.LoadCasbinRulesRow) []string {
	values := []pgtype.Text{row.V0, row.V1, row.V2, row.V3, row.V4, row.V5}
	rule := make([]string, 0, maxValues)
	for _, v := range values {
		if !v.Valid {
			break
		}
		rule = append(rule, v.String)
	}
	return rule
}

// querierWithTx runs fn with a querier bound to a new transaction and
// commits it unless fn returns an error. It requires db to start
// transactions, which both *pgx.Conn and *pgxpool.Pool do.
func (a *CasbinAdapter) querierWithTx(ctx context.Context, fn func(q sqlc.Querier) error) error {
	tx, err := a.begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	if err := fn(sqlc.New(tx)); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			return fmt.Errorf("rollback: %w (original error: %v)", rbErr, err)
		}
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// begin starts a new transaction from db.
func (a *CasbinAdapter) begin(ctx context.Context) (pgx.Tx, error) {
	b, ok := a.db.(interface {
		Begin(context.Context) (pgx.Tx, error)
	})
	if !ok {
		return nil, fmt.Errorf("adapter db %T cannot start transactions", a.db)
	}
	return b.Begin(ctx)
}

// checkSection rejects anything but the policy and grouping sections casbin
// manages.
func checkSection(sec string) error {
	if sec != "p" && sec != "g" {
		return fmt.Errorf("section %q is not a casbin section", sec)
	}
	return nil
}

// LoadPolicy loads every stored rule into m. It implements casbin's
// persist.Adapter.
func (a *CasbinAdapter) LoadPolicy(m model.Model) error {
	ctx := context.Background()
	rows, err := sqlc.New(a.db).LoadCasbinRules(ctx)
	if err != nil {
		return fmt.Errorf("load casbin rules: %w", err)
	}
	a.filtered = false
	return applyRows(rows, m)
}

// LoadFilteredPolicy loads the stored rules into m, keeping only the ptype
// given as filter. A nil filter loads everything; any other kind is an
// error. It implements casbin's persist.FilteredAdapter.
func (a *CasbinAdapter) LoadFilteredPolicy(m model.Model, filter interface{}) error {
	if filter == nil {
		return a.LoadPolicy(m)
	}
	ptype, ok := filter.(string)
	if !ok {
		return fmt.Errorf("filter %v is not a ptype string", filter)
	}
	ctx := context.Background()
	rows, err := sqlc.New(a.db).LoadCasbinRulesByPtype(ctx, ptype)
	if err != nil {
		return fmt.Errorf("load casbin rules of ptype %q: %w", ptype, err)
	}
	a.filtered = true
	loaded := make([]sqlc.LoadCasbinRulesRow, 0, len(rows))
	for _, row := range rows {
		loaded = append(loaded, fromPtypeRow(row))
	}
	return applyRows(loaded, m)
}

// fromPtypeRow converts a filtered-load row into a full-load row.
func fromPtypeRow(row sqlc.LoadCasbinRulesByPtypeRow) sqlc.LoadCasbinRulesRow {
	return sqlc.LoadCasbinRulesRow{
		Ptype: row.Ptype,
		V0:    row.V0, V1: row.V1, V2: row.V2,
		V3: row.V3, V4: row.V4, V5: row.V5,
	}
}

func applyRows(rows []sqlc.LoadCasbinRulesRow, m model.Model) error {
	for _, row := range rows {
		line := append([]string{row.Ptype}, ruleFromRow(row)...)
		if err := persist.LoadPolicyArray(line, m); err != nil {
			return fmt.Errorf("apply rule %v: %w", line, err)
		}
	}
	return nil
}

// SavePolicy replaces every stored rule with the rules of m, in one
// transaction. It implements casbin's persist.Adapter.
func (a *CasbinAdapter) SavePolicy(m model.Model) error {
	if err := a.querierWithTx(context.Background(), func(q sqlc.Querier) error {
		if err := q.DeleteAllCasbinRules(context.Background()); err != nil {
			return fmt.Errorf("delete all casbin rules: %w", err)
		}
		// Every ptype of both sections, not only p and g: a model with
		// p2 or g2 (domains, resource roles) must not lose those rules.
		for _, sec := range []string{"p", "g"} {
			for ptype, assertion := range m[sec] {
				for _, rule := range assertion.Policy {
					if err := a.insertRule(q, ptype, rule); err != nil {
						return err
					}
				}
			}
		}
		return nil
	}); err != nil {
		return fmt.Errorf("save casbin policy: %w", err)
	}
	return nil
}

// AddPolicy adds one rule to the storage. It implements casbin's
// persist.Adapter.
func (a *CasbinAdapter) AddPolicy(sec string, ptype string, rule []string) error {
	if err := checkSection(sec); err != nil {
		return err
	}
	return a.insertRules(ptype, [][]string{rule})
}

// AddPolicies adds several rules at once, in one transaction. It
// implements casbin's persist.BatchAdapter.
func (a *CasbinAdapter) AddPolicies(sec string, ptype string, rules [][]string) error {
	if err := checkSection(sec); err != nil {
		return err
	}
	return a.insertRules(ptype, rules)
}

func (a *CasbinAdapter) insertRules(ptype string, rules [][]string) error {
	if err := a.querierWithTx(context.Background(), func(q sqlc.Querier) error {
		for _, rule := range rules {
			if err := a.insertRule(q, ptype, rule); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return fmt.Errorf("add casbin %q rules: %w", ptype, err)
	}
	return nil
}

func (a *CasbinAdapter) insertRule(q sqlc.Querier, ptype string, rule []string) error {
	values, err := valuesFromRule(rule)
	if err != nil {
		return err
	}
	if err := q.InsertCasbinRule(context.Background(), sqlc.InsertCasbinRuleParams{
		Ptype: ptype,
		V0:    values[0], V1: values[1], V2: values[2],
		V3: values[3], V4: values[4], V5: values[5],
	}); err != nil {
		return fmt.Errorf("insert casbin rule %q %v: %w", ptype, rule, err)
	}
	return nil
}

// IsFiltered reports whether the most recent load was filtered. It
// implements casbin's persist.FilteredAdapter. It is not safe for
// concurrent use by two enforcers sharing one adapter.
func (a *CasbinAdapter) IsFiltered() bool {
	return a.filtered
}

// removeValues pads or bounds rule to maxValues pgtype.Text arguments for
// a delete.
func removeValues(ptype string, rule []string) (sqlc.DeleteCasbinRuleParams, error) {
	values, err := valuesFromRule(rule)
	if err != nil {
		return sqlc.DeleteCasbinRuleParams{}, err
	}
	return sqlc.DeleteCasbinRuleParams{
		Ptype: ptype,
		V0:    values[0], V1: values[1], V2: values[2],
		V3: values[3], V4: values[4], V5: values[5],
	}, nil
}

// RemovePolicy removes one exact rule from the storage. It implements
// casbin's persist.Adapter.
func (a *CasbinAdapter) RemovePolicy(sec string, ptype string, rule []string) error {
	if err := checkSection(sec); err != nil {
		return err
	}
	params, err := removeValues(ptype, rule)
	if err != nil {
		return err
	}
	ctx := context.Background()
	if err := sqlc.New(a.db).DeleteCasbinRule(ctx, params); err != nil {
		return fmt.Errorf("remove casbin rule %q %v: %w", ptype, rule, err)
	}
	return nil
}

// RemovePolicies removes several rules at once, in one transaction. It
// implements casbin's persist.BatchAdapter.
func (a *CasbinAdapter) RemovePolicies(sec string, ptype string, rules [][]string) error {
	if err := checkSection(sec); err != nil {
		return err
	}
	return a.querierWithTx(context.Background(), func(q sqlc.Querier) error {
		for _, rule := range rules {
			params, err := removeValues(ptype, rule)
			if err != nil {
				return err
			}
			if err := q.DeleteCasbinRule(context.Background(), params); err != nil {
				return fmt.Errorf("remove casbin rule %q %v: %w", ptype, rule, err)
			}
		}
		return nil
	})
}

// RemoveFilteredPolicy removes every rule of ptype whose values match
// fieldValues starting at fieldIndex. As in Casbin's own model, an empty
// field value is a wildcard, and so is every column outside the filter, so
// no fieldValues removes every rule of the ptype. It implements casbin's
// persist.Adapter.
func (a *CasbinAdapter) RemoveFilteredPolicy(sec string, ptype string, fieldIndex int, fieldValues ...string) error {
	if err := checkSection(sec); err != nil {
		return err
	}
	if fieldIndex < 0 || fieldIndex >= maxValues {
		return fmt.Errorf("field index %d is out of range 0-%d", fieldIndex, maxValues-1)
	}
	if fieldIndex+len(fieldValues) > maxValues {
		return fmt.Errorf("field index %d plus %d values exceeds the %d value columns", fieldIndex, len(fieldValues), maxValues)
	}
	fields := make([]pgtype.Text, maxValues) // all NULL: match anything
	for i, v := range fieldValues {
		if v == "" {
			continue // Casbin's wildcard
		}
		fields[fieldIndex+i] = pgtype.Text{String: v, Valid: true}
	}
	ctx := context.Background()
	err := sqlc.New(a.db).DeleteCasbinRulesFiltered(ctx, sqlc.DeleteCasbinRulesFilteredParams{
		Ptype: ptype,
		F0:    fields[0], F1: fields[1], F2: fields[2],
		F3: fields[3], F4: fields[4], F5: fields[5],
	})
	if err != nil {
		return fmt.Errorf("remove filtered casbin rules of ptype %q: %w", ptype, err)
	}
	return nil
}
