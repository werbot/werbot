package pg

import (
	"slices"
	"strings"

	pgq "github.com/pganalyze/pg_query_go/v6"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/werbot/werbot/internal/platform/shellclass"
	"github.com/werbot/werbot/internal/platform/sqlclass"
)

// builtinSecrets are the catalog tables that hold credentials.
var builtinSecrets = []string{
	"pg_catalog.pg_authid", "pg_catalog.pg_shadow", "pg_catalog.pg_user_mapping", "pg_catalog.pg_user_mappings",
}

// settingsTable changes session settings when written (as set_config does).
const settingsTable = "pg_catalog.pg_settings"

// refs walks whole statement trees for what statement nodes alone do not
// show: the tables read and written and the classes of function calls.
type refs struct {
	tables, writes []string
	funcs          set
	ctes           []string // CTE names in scope
	dry            bool     // inside EXPLAIN without ANALYZE: nothing runs
	lock           bool     // inside the FROM clause of SELECT … FOR UPDATE/SHARE
}

// visitors handle the nodes that name write targets or open a CTE scope; the
// walk descends into every other node field by field.
var visitors map[protoreflect.Name]func(*refs, protoreflect.Message)

func init() {
	visitors = map[protoreflect.Name]func(*refs, protoreflect.Message){
		"RangeVar":      func(r *refs, m protoreflect.Message) { r.ref(m.Interface().(*pgq.RangeVar)) },
		"LockingClause": func(*refs, protoreflect.Message) {}, // OF names are aliases; FROM lists the tables
		"FuncCall":      (*refs).funcCall,
		"ExplainStmt":   (*refs).explain,
		"SelectStmt":    (*refs).selectStmt,
		"InsertStmt":    (*refs).targetStmt,
		"UpdateStmt":    (*refs).targetStmt,
		"DeleteStmt":    (*refs).targetStmt,
		"MergeStmt":     (*refs).targetStmt,
		"CopyStmt":      (*refs).copyStmt,
		"AlterTableStmt": func(r *refs, m protoreflect.Message) {
			r.target(m.Interface().(*pgq.AlterTableStmt).GetRelation())
			r.fields(m, "relation")
		},
		"RenameStmt": func(r *refs, m protoreflect.Message) {
			r.target(m.Interface().(*pgq.RenameStmt).GetRelation())
			r.fields(m, "relation")
		},
		"IntoClause": func(r *refs, m protoreflect.Message) {
			r.target(m.Interface().(*pgq.IntoClause).GetRel())
			r.fields(m, "rel")
		},
		"TruncateStmt": func(r *refs, m protoreflect.Message) {
			for _, n := range m.Interface().(*pgq.TruncateStmt).GetRelations() {
				r.target(n.GetRangeVar())
			}
		},
		"DropStmt": (*refs).drop,
	}
}

func (r *refs) visit(m protoreflect.Message) {
	if v, ok := visitors[m.Descriptor().Name()]; ok {
		v(r, m)
		return
	}
	r.fields(m)
}

// fields visits the message fields of m but the skipped ones.
func (r *refs) fields(m protoreflect.Message, skip ...protoreflect.Name) {
	m.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		switch {
		case fd.Kind() != protoreflect.MessageKind || slices.Contains(skip, fd.Name()):
		case fd.IsList():
			for i, l := 0, v.List(); i < l.Len(); i++ {
				r.visit(l.Get(i).Message())
			}
		default:
			r.visit(v.Message())
		}
		return true
	})
}

// ref records a table the text reads; a name without a schema may be a CTE.
func (r *refs) ref(v *pgq.RangeVar) {
	if v.GetSchemaname() == "" && slices.Contains(r.ctes, v.GetRelname()) {
		return
	}
	r.add(relName(v), r.lock)
}

// target records a table a statement changes; a target is never a CTE.
func (r *refs) target(v *pgq.RangeVar) {
	if v != nil {
		r.add(relName(v), true)
	}
}

func (r *refs) add(name string, write bool) {
	r.tables = append(r.tables, name)
	if write && !r.dry {
		r.writes = append(r.writes, name)
	}
}

func (r *refs) funcCall(m protoreflect.Message) {
	if f := m.Interface().(*pgq.FuncCall); !r.dry {
		r.funcs = sqlclass.Merge(r.funcs, set{funcClass(strs(f.GetFuncname()), f.GetArgs())})
	}
	r.fields(m)
}

func (r *refs) explain(m protoreflect.Message) {
	dry := r.dry
	r.dry = dry || !analyzed(m.Interface().(*pgq.ExplainStmt))
	r.fields(m)
	r.dry = dry
}

// with brings the CTE names of w into scope for the rest of its statement:
// a CTE sees the earlier ones, all of them under RECURSIVE. The caller
// restores the returned scope length when the statement ends.
func (r *refs) with(w *pgq.WithClause) int {
	n := len(r.ctes)
	ctes := w.GetCtes()
	if w.GetRecursive() {
		for _, c := range ctes {
			r.ctes = append(r.ctes, c.GetCommonTableExpr().GetCtename())
		}
	}
	for _, c := range ctes {
		cte := c.GetCommonTableExpr()
		r.visit(cte.ProtoReflect())
		if !w.GetRecursive() {
			r.ctes = append(r.ctes, cte.GetCtename())
		}
	}
	return n
}

func (r *refs) selectStmt(m protoreflect.Message) {
	s := m.Interface().(*pgq.SelectStmt)
	defer r.scope(r.with(s.GetWithClause()))
	if len(s.GetLockingClause()) == 0 {
		r.fields(m, "with_clause")
		return
	}
	r.fields(m, "with_clause", "from_clause")
	lock := r.lock
	r.lock = true
	for _, n := range s.GetFromClause() {
		r.visit(n.ProtoReflect())
	}
	r.lock = lock
}

// targetStmt walks INSERT, UPDATE, DELETE and MERGE: relation is the target.
func (r *refs) targetStmt(m protoreflect.Message) {
	var w *pgq.WithClause
	var rel *pgq.RangeVar
	switch s := m.Interface().(type) {
	case *pgq.InsertStmt:
		w, rel = s.GetWithClause(), s.GetRelation()
	case *pgq.UpdateStmt:
		w, rel = s.GetWithClause(), s.GetRelation()
	case *pgq.DeleteStmt:
		w, rel = s.GetWithClause(), s.GetRelation()
	case *pgq.MergeStmt:
		w, rel = s.GetWithClause(), s.GetRelation()
	}
	defer r.scope(r.with(w))
	r.target(rel)
	r.fields(m, "with_clause", "relation")
}

func (r *refs) scope(n int) { r.ctes = r.ctes[:n] }

func (r *refs) copyStmt(m protoreflect.Message) {
	s := m.Interface().(*pgq.CopyStmt)
	if s.GetIsFrom() {
		r.target(s.GetRelation())
	} else if s.GetRelation() != nil {
		r.add(relName(s.GetRelation()), false)
	}
	r.fields(m, "relation")
}

// drop records dropped tables and views; their names are string lists.
func (r *refs) drop(m protoreflect.Message) {
	s := m.Interface().(*pgq.DropStmt)
	switch s.GetRemoveType() {
	case pgq.ObjectType_OBJECT_TABLE, pgq.ObjectType_OBJECT_VIEW, pgq.ObjectType_OBJECT_MATVIEW,
		pgq.ObjectType_OBJECT_FOREIGN_TABLE:
	default:
		return
	}
	for _, o := range s.GetObjects() {
		names := strs(o.GetList().GetItems())
		if len(names) == 1 {
			names = append([]string{""}, names...)
		}
		r.add(qualify(names[len(names)-2], names[len(names)-1]), true)
	}
}

// result adds the function classes and the table rules to the statement
// classes; it sorts the table lists.
func (r *refs) result(classes set, statements int) sqlclass.Result {
	out := sqlclass.Result{Classes: plus(classes, r.funcs), Tables: sorted(r.tables), WriteTables: sorted(r.writes), Statements: statements}
	if slices.Contains(out.WriteTables, settingsTable) {
		out.Classes = sqlclass.Merge(out.Classes, set{shellclass.Privilege})
	}
	return sqlclass.MarkSecrets(out, builtinSecrets)
}

func sorted(s []string) []string {
	slices.Sort(s)
	return slices.Compact(s)
}

// relName is "schema.table"; without a schema a pg_* name is in pg_catalog,
// any other in public.
func relName(rv *pgq.RangeVar) string { return qualify(rv.GetSchemaname(), rv.GetRelname()) }

func qualify(schema, name string) string {
	switch {
	case schema != "":
	case strings.HasPrefix(name, "pg_"):
		schema = "pg_catalog"
	default:
		schema = "public"
	}
	return schema + "." + name
}
