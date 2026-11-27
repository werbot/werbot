package pg

import (
	"strings"

	pgq "github.com/pganalyze/pg_query_go/v6"

	"github.com/werbot/werbot/internal/platform/shellclass"
	"github.com/werbot/werbot/internal/platform/sqlclass"
)

// setConfig changes a session setting; its class depends on the setting name.
const setConfig = "set_config"

// egressPrefix names the dblink extension functions (outside pg_catalog).
const egressPrefix = "dblink"

// functions maps the built-in functions of pg_catalog to their class. A call
// of any other function, or of a function of another schema, is opaque.
var functions = classTable(map[shellclass.Class][]string{
	shellclass.Read: {
		// aggregates and window functions
		"count", "sum", "avg", "min", "max", "array_agg", "string_agg", "json_agg", "jsonb_agg",
		"json_object_agg", "jsonb_object_agg", "bool_and", "bool_or", "every", "bit_and", "bit_or",
		"stddev", "stddev_pop", "stddev_samp", "variance", "var_pop", "var_samp", "corr",
		"covar_pop", "covar_samp", "percentile_cont", "percentile_disc", "mode",
		"row_number", "rank", "dense_rank", "percent_rank", "cume_dist", "ntile", "lag", "lead",
		"first_value", "last_value", "nth_value",
		// mathematics
		"abs", "ceil", "ceiling", "floor", "round", "trunc", "sign", "sqrt", "cbrt", "power", "exp",
		"ln", "log", "log10", "mod", "div", "pi", "degrees", "radians", "random", "width_bucket",
		"gcd", "lcm", "factorial", "sin", "cos", "tan", "asin", "acos", "atan", "atan2",
		// strings (and the functions the grammar calls for LIKE, SIMILAR TO, TRIM, ...)
		"length", "char_length", "character_length", "octet_length", "bit_length", "lower", "upper",
		"initcap", "concat", "concat_ws", "format", "left", "right", "lpad", "rpad", "ltrim", "rtrim",
		"btrim", "replace", "reverse", "repeat", "split_part", "strpos", "substr", "substring",
		"position", "overlay", "starts_with", "translate", "ascii", "chr", "md5", "sha224", "sha256",
		"sha384", "sha512", "encode", "decode", "to_hex", "quote_ident", "quote_literal",
		"quote_nullable", "regexp_match", "regexp_matches", "regexp_replace", "regexp_split_to_array",
		"regexp_split_to_table", "regexp_count", "regexp_instr", "regexp_like", "regexp_substr",
		"string_to_array", "array_to_string", "to_char", "to_number", "like_escape",
		"similar_to_escape", "normalize", "is_normalized", "pg_collation_for", "num_nulls", "num_nonnulls",
		// date and time
		"now", "clock_timestamp", "statement_timestamp", "transaction_timestamp", "timeofday",
		"date_trunc", "date_part", "extract", "age", "make_date", "make_time", "make_timestamp",
		"make_timestamptz", "make_interval", "date_bin", "justify_days", "justify_hours",
		"justify_interval", "isfinite", "timezone", "overlaps", "to_date", "to_timestamp",
		// JSON
		"json_build_object", "jsonb_build_object", "json_build_array", "jsonb_build_array", "to_json",
		"to_jsonb", "row_to_json", "array_to_json", "json_object", "jsonb_object",
		"json_array_length", "jsonb_array_length", "json_each", "jsonb_each", "json_each_text",
		"jsonb_each_text", "json_extract_path", "jsonb_extract_path", "json_extract_path_text",
		"jsonb_extract_path_text", "json_object_keys", "jsonb_object_keys", "json_array_elements",
		"jsonb_array_elements", "json_array_elements_text", "jsonb_array_elements_text", "json_typeof",
		"jsonb_typeof", "jsonb_set", "jsonb_insert", "json_strip_nulls", "jsonb_strip_nulls",
		"jsonb_pretty", "jsonb_path_query", "jsonb_path_exists", "jsonb_path_match",
		"jsonb_path_query_array", "jsonb_path_query_first", "json_populate_record",
		"jsonb_populate_record", "json_to_record", "jsonb_to_record", "json_to_recordset",
		"jsonb_to_recordset",
		// arrays
		"array_length", "array_lower", "array_upper", "array_dims", "array_ndims", "cardinality",
		"array_append", "array_prepend", "array_cat", "array_remove", "array_replace",
		"array_position", "array_positions", "array_fill", "trim_array", "unnest", "generate_subscripts",
		// full-text search
		"to_tsvector", "to_tsquery", "plainto_tsquery", "phraseto_tsquery", "websearch_to_tsquery",
		"ts_rank", "ts_rank_cd", "ts_headline", "setweight", "strip", "numnode", "querytree",
		"tsvector_to_array",
		// session and catalog information
		"generate_series", "gen_random_uuid", "uuidv7", "current_setting", "current_database",
		"current_schema", "current_schemas", "version", "pg_typeof", "format_type", "currval", "lastval",
		"has_any_column_privilege", "has_column_privilege", "has_database_privilege",
		"has_foreign_data_wrapper_privilege", "has_function_privilege", "has_language_privilege",
		"has_parameter_privilege", "has_schema_privilege", "has_sequence_privilege",
		"has_server_privilege", "has_table_privilege", "has_tablespace_privilege", "has_type_privilege",
		"pg_column_size", "pg_database_size", "pg_indexes_size", "pg_relation_size", "pg_table_size",
		"pg_tablespace_size", "pg_total_relation_size", "pg_size_pretty", "pg_size_bytes",
		"pg_get_constraintdef", "pg_get_functiondef", "pg_get_indexdef", "pg_get_ruledef",
		"pg_get_triggerdef", "pg_get_viewdef", "pg_get_statisticsobjdef", "pg_get_partkeydef",
		"pg_get_expr", "pg_get_userbyid", "pg_get_serial_sequence", "obj_description",
		"col_description", "shobj_description", "pg_backend_pid", "pg_is_in_recovery",
		"pg_postmaster_start_time", "pg_conf_load_time", "pg_sleep", "pg_sleep_for", "pg_sleep_until",
		"to_regclass", "to_regtype", "to_regproc", "to_regnamespace", "to_regrole",
	},
	shellclass.Write: {
		"nextval", "setval", "pg_advisory_lock", "pg_advisory_lock_shared", "pg_advisory_xact_lock",
		"pg_advisory_xact_lock_shared", "pg_notify", "lo_create", "lo_unlink", "lo_put", "lo_from_bytea",
	},
	shellclass.Privilege: {
		"pg_terminate_backend", "pg_cancel_backend", "pg_reload_conf", "pg_rotate_logfile",
		"pg_promote", "pg_switch_wal", "pg_create_restore_point", "pg_create_physical_replication_slot",
		"pg_create_logical_replication_slot", "pg_drop_replication_slot", "pg_stat_reset",
		"pg_stat_reset_shared", "pg_stat_reset_single_table_counters",
		"pg_stat_reset_single_function_counters", "pg_stat_reset_slru",
		"pg_stat_reset_replication_slot", "pg_stat_reset_subscription_stats",
	},
	shellclass.Secrets: {
		"pg_read_file", "pg_read_binary_file", "pg_ls_dir", "pg_stat_file", "pg_ls_logdir",
		"pg_ls_waldir", "pg_ls_tmpdir", "pg_ls_archive_statusdir",
	},
	shellclass.Egress: {"lo_import", "lo_export"},
})

func classTable(byClass map[shellclass.Class][]string) map[string]shellclass.Class {
	out := map[string]shellclass.Class{}
	for c, names := range byClass {
		for _, n := range names {
			out[n] = c
		}
	}
	return out
}

// funcClass classes a call of the function names (as written, the schema
// first when given) with args.
func funcClass(names []string, args []*pgq.Node) shellclass.Class {
	switch {
	case len(names) == 2 && names[0] == "pg_catalog":
	case len(names) != 1:
		return shellclass.Opaque
	}
	name := names[len(names)-1]
	if c, ok := functions[name]; ok {
		return c
	}
	switch {
	case name == setConfig:
		return setConfigClass(args)
	case strings.HasPrefix(name, egressPrefix):
		return shellclass.Egress
	}
	return shellclass.Opaque
}

// setConfigClass: a protected setting, or a name that is not a string
// constant, is privilege.
func setConfigClass(args []*pgq.Node) shellclass.Class {
	if len(args) == 0 {
		return shellclass.Privilege
	}
	name := args[0].GetAConst().GetSval()
	if name == nil || sqlclass.Protected(name.GetSval()) {
		return shellclass.Privilege
	}
	return shellclass.Read
}

// strs returns the String values of nodes.
func strs(nodes []*pgq.Node) []string {
	out := make([]string, len(nodes))
	for i, n := range nodes {
		out[i] = n.GetString_().GetSval()
	}
	return out
}
