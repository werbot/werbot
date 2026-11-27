package pg

import (
	"slices"
	"strings"

	pgq "github.com/pganalyze/pg_query_go/v6"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/werbot/werbot/internal/platform/shellclass"
	"github.com/werbot/werbot/internal/platform/sqlclass"
)

// set is a class list as sqlclass.Result holds it.
type set = []shellclass.Class

var (
	cRead        = set{shellclass.Read}
	cWrite       = set{shellclass.Write}
	cDestructive = set{shellclass.Destructive}
	cPrivilege   = set{shellclass.Privilege}
	cEgress      = set{shellclass.Egress}
	cOpaque      = set{shellclass.Opaque}
)

// fixed maps statement nodes to constant classes. A node in neither fixed
// nor special is opaque (DoStmt, CallStmt, ExecuteStmt among them).
var fixed = map[protoreflect.Name]set{
	"VariableShowStmt": cRead, "FetchStmt": cRead, "ClosePortalStmt": cRead,
	"DeallocateStmt": cRead, "ListenStmt": cRead, "UnlistenStmt": cRead,

	"NotifyStmt": cWrite, "LockStmt": cWrite, "VacuumStmt": cWrite, "ClusterStmt": cWrite,
	"ReindexStmt": cWrite, "RefreshMatViewStmt": cWrite, "RenameStmt": cWrite, "CommentStmt": cWrite,
	"CreateStmt": cWrite, "IndexStmt": cWrite, "CreateFunctionStmt": cWrite, "CreateTrigStmt": cWrite,
	"CreateSeqStmt": cWrite, "CreatedbStmt": cWrite,

	"DropStmt": cDestructive, "TruncateStmt": cDestructive, "DropdbStmt": cDestructive,
	"DropTableSpaceStmt": cDestructive,

	"DiscardStmt": cPrivilege, "GrantStmt": cPrivilege, "GrantRoleStmt": cPrivilege,
	"CreateRoleStmt": cPrivilege, "AlterRoleStmt": cPrivilege, "AlterRoleSetStmt": cPrivilege,
	"AlterDatabaseSetStmt": cPrivilege, "AlterSystemStmt": cPrivilege,
	"AlterDefaultPrivilegesStmt": cPrivilege, "AlterOwnerStmt": cPrivilege,
	"ReassignOwnedStmt": cPrivilege, "CreateExtensionStmt": cPrivilege,
	"AlterExtensionStmt": cPrivilege, "LoadStmt": cPrivilege, "CreatePolicyStmt": cPrivilege,
	"AlterPolicyStmt": cPrivilege, "SecLabelStmt": cPrivilege, "CheckPointStmt": cPrivilege,
	"DropRoleStmt":  {shellclass.Destructive, shellclass.Privilege},
	"DropOwnedStmt": {shellclass.Destructive, shellclass.Privilege},

	"CreateForeignServerStmt": cEgress, "CreateUserMappingStmt": cEgress,
	"ImportForeignSchemaStmt": cEgress,
}

// special maps statement nodes whose classes depend on their fields. It is
// filled in init: its functions recurse into stmtClasses, which reads it.
var special map[protoreflect.Name]func(*pgq.Node) set

func init() {
	special = map[protoreflect.Name]func(*pgq.Node) set{
		"SelectStmt":        func(n *pgq.Node) set { return selectClasses(n.GetSelectStmt()) },
		"TransactionStmt":   func(n *pgq.Node) set { return transactionClasses(n.GetTransactionStmt()) },
		"VariableSetStmt":   func(n *pgq.Node) set { return setClasses(n.GetVariableSetStmt()) },
		"ExplainStmt":       func(n *pgq.Node) set { return explainClasses(n.GetExplainStmt()) },
		"InsertStmt":        insertClasses,
		"MergeStmt":         func(n *pgq.Node) set { return plus(cWrite, with(n.GetMergeStmt().GetWithClause())) },
		"UpdateStmt":        func(n *pgq.Node) set { s := n.GetUpdateStmt(); return filtered(s.GetWhereClause(), s.GetWithClause()) },
		"DeleteStmt":        func(n *pgq.Node) set { s := n.GetDeleteStmt(); return filtered(s.GetWhereClause(), s.GetWithClause()) },
		"CreateTableAsStmt": func(n *pgq.Node) set { return plus(cWrite, stmtClasses(n.GetCreateTableAsStmt().GetQuery())) },
		"ViewStmt":          func(n *pgq.Node) set { return plus(cWrite, stmtClasses(n.GetViewStmt().GetQuery())) },
		"AlterTableStmt":    func(n *pgq.Node) set { return alterTableClasses(n.GetAlterTableStmt()) },
		"CopyStmt":          func(n *pgq.Node) set { return copyClasses(n.GetCopyStmt()) },
		"PrepareStmt":       func(n *pgq.Node) set { return stmtClasses(n.GetPrepareStmt().GetQuery()) },
		"DeclareCursorStmt": func(n *pgq.Node) set { return stmtClasses(n.GetDeclareCursorStmt().GetQuery()) },
	}
}

var nodeOneof = (&pgq.Node{}).ProtoReflect().Descriptor().Oneofs().ByName("node")

// classes is the union of the statement classes of tree; no statement is read.
func classes(tree *pgq.ParseResult) sqlclass.Result {
	stmts := tree.GetStmts()
	if len(stmts) == 0 {
		return sqlclass.Result{Classes: cRead}
	}
	var out set
	for _, s := range stmts {
		out = sqlclass.Merge(out, stmtClasses(s.GetStmt()))
	}
	return sqlclass.Result{Classes: out, Statements: len(stmts)}
}

func stmtClasses(n *pgq.Node) set {
	fd := n.ProtoReflect().WhichOneof(nodeOneof)
	if fd == nil {
		return cOpaque
	}
	name := fd.Message().Name()
	if f, ok := special[name]; ok {
		return f(n)
	}
	if c, ok := fixed[name]; ok {
		return c
	}
	return cOpaque
}

// plus merges nested classes into base; a nested read adds nothing to a
// statement that changes data.
func plus(base set, nested ...set) set {
	out := base
	for _, c := range nested {
		out = sqlclass.Merge(out, slices.DeleteFunc(slices.Clone(c), func(x shellclass.Class) bool { return x == shellclass.Read }))
	}
	return out
}

func insertClasses(n *pgq.Node) set {
	s := n.GetInsertStmt()
	out := plus(cWrite, with(s.GetWithClause()))
	if q := s.GetSelectStmt(); q != nil { // nil for DEFAULT VALUES
		out = plus(out, stmtClasses(q))
	}
	return out
}

func with(w *pgq.WithClause) set {
	var out set
	for _, cte := range w.GetCtes() {
		out = sqlclass.Merge(out, stmtClasses(cte.GetCommonTableExpr().GetCtequery()))
	}
	return out
}

func selectClasses(s *pgq.SelectStmt) set {
	base := cRead
	if s.GetIntoClause() != nil {
		base = cWrite
	}
	if len(s.GetLockingClause()) > 0 {
		base = sqlclass.Merge(base, cWrite)
	}
	out := plus(base, with(s.GetWithClause()))
	for _, arm := range []*pgq.SelectStmt{s.GetLarg(), s.GetRarg()} {
		if arm != nil {
			out = sqlclass.Merge(out, selectClasses(arm))
		}
	}
	return out
}

// filtered classes UPDATE and DELETE: without WHERE every row changes.
func filtered(where *pgq.Node, w *pgq.WithClause) set {
	if where == nil {
		return plus(cDestructive, with(w))
	}
	return plus(cWrite, with(w))
}

// transactionClasses: READ WRITE overrides a read-only session default;
// prepared transactions act on other transactions.
func transactionClasses(s *pgq.TransactionStmt) set {
	switch s.GetKind() {
	case pgq.TransactionStmtKind_TRANS_STMT_PREPARE, pgq.TransactionStmtKind_TRANS_STMT_COMMIT_PREPARED,
		pgq.TransactionStmtKind_TRANS_STMT_ROLLBACK_PREPARED:
		return cWrite
	}
	if readWrite(s.GetOptions()) {
		return cPrivilege
	}
	return cRead
}

func setClasses(s *pgq.VariableSetStmt) set {
	if s.GetKind() == pgq.VariableSetKind_VAR_RESET_ALL || sqlclass.Protected(s.GetName()) ||
		(s.GetKind() == pgq.VariableSetKind_VAR_SET_MULTI && readWrite(s.GetArgs())) {
		return cPrivilege
	}
	return cRead
}

// readWrite reports a transaction_read_only option that is not a true constant.
func readWrite(opts []*pgq.Node) bool {
	for _, o := range opts {
		d := o.GetDefElem()
		if d.GetDefname() == "transaction_read_only" && d.GetArg().GetAConst().GetIval().GetIval() == 0 {
			return true
		}
	}
	return false
}

// explainClasses: EXPLAIN without ANALYZE does not run the statement.
func explainClasses(s *pgq.ExplainStmt) set {
	for _, o := range s.GetOptions() {
		if d := o.GetDefElem(); d.GetDefname() == "analyze" && !isFalse(d.GetArg()) {
			return stmtClasses(s.GetQuery())
		}
	}
	return cRead
}

// isFalse reports an option argument PostgreSQL reads as false; anything
// else, including an invalid value, counts as true.
func isFalse(arg *pgq.Node) bool {
	if s := arg.GetString_(); s != nil {
		return strings.EqualFold(s.GetSval(), "false") || strings.EqualFold(s.GetSval(), "off")
	}
	if i := arg.GetInteger(); i != nil {
		return i.GetIval() == 0
	}
	return false
}

func alterTableClasses(s *pgq.AlterTableStmt) set {
	var out set
	for _, c := range s.GetCmds() {
		out = sqlclass.Merge(out, alterCmdClasses(c.GetAlterTableCmd().GetSubtype()))
	}
	return out
}

func alterCmdClasses(t pgq.AlterTableType) set {
	name := t.String()
	switch {
	case strings.HasPrefix(name, "AT_Drop"):
		return cDestructive
	case strings.HasSuffix(name, "RowSecurity"), t == pgq.AlterTableType_AT_ChangeOwner:
		return cPrivilege
	}
	return cWrite
}

// copyClasses: a server file or program is egress; STDIN writes, STDOUT reads.
func copyClasses(s *pgq.CopyStmt) set {
	base := cRead
	if s.GetIsFrom() {
		base = cWrite
	}
	if s.GetIsProgram() || s.GetFilename() != "" {
		base = sqlclass.Merge(base, cEgress)
		if !s.GetIsFrom() {
			base = cEgress
		}
	}
	if q := s.GetQuery(); q != nil {
		return sqlclass.Merge(base, plus(nil, stmtClasses(q)))
	}
	return base
}
