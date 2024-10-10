package access

import (
	"google.golang.org/protobuf/proto"

	schemeaccesspb "github.com/werbot/werbot/internal/core/scheme/proto/access"
)

func NewDatabaseMySQL() proto.Message { return &schemeaccesspb.AccessScheme_Database_MySQL{} }
func SetDatabaseMySQL(core *schemeaccesspb.AccessScheme, msg proto.Message) {
	core.Access = &schemeaccesspb.AccessScheme_DatabaseMysql{DatabaseMysql: msg.(*schemeaccesspb.AccessScheme_Database_MySQL)}
}

func NewDatabasePostgres() proto.Message { return &schemeaccesspb.AccessScheme_Database_Postgres{} }
func SetDatabasePostgres(core *schemeaccesspb.AccessScheme, msg proto.Message) {
	core.Access = &schemeaccesspb.AccessScheme_DatabasePostgres{DatabasePostgres: msg.(*schemeaccesspb.AccessScheme_Database_Postgres)}
}

func NewDatabaseRedis() proto.Message { return &schemeaccesspb.AccessScheme_Database_Redis{} }
func SetDatabaseRedis(core *schemeaccesspb.AccessScheme, msg proto.Message) {
	core.Access = &schemeaccesspb.AccessScheme_DatabaseRedis{DatabaseRedis: msg.(*schemeaccesspb.AccessScheme_Database_Redis)}
}

func NewDatabaseMongoDB() proto.Message { return &schemeaccesspb.AccessScheme_Database_MongoDB{} }
func SetDatabaseMongoDB(core *schemeaccesspb.AccessScheme, msg proto.Message) {
	core.Access = &schemeaccesspb.AccessScheme_DatabaseMongodb{DatabaseMongodb: msg.(*schemeaccesspb.AccessScheme_Database_MongoDB)}
}

func NewDatabaseElastic() proto.Message { return &schemeaccesspb.AccessScheme_Database_Elastic{} }
func SetDatabaseElastic(core *schemeaccesspb.AccessScheme, msg proto.Message) {
	core.Access = &schemeaccesspb.AccessScheme_DatabaseElastic{DatabaseElastic: msg.(*schemeaccesspb.AccessScheme_Database_Elastic)}
}

func NewDatabaseDynamoDB() proto.Message { return &schemeaccesspb.AccessScheme_Database_DynamoDB{} }
func SetDatabaseDynamoDB(core *schemeaccesspb.AccessScheme, msg proto.Message) {
	core.Access = &schemeaccesspb.AccessScheme_DatabaseDynamodb{DatabaseDynamodb: msg.(*schemeaccesspb.AccessScheme_Database_DynamoDB)}
}

func NewDatabaseCassandra() proto.Message { return &schemeaccesspb.AccessScheme_Database_Cassandra{} }
func SetDatabaseCassandra(core *schemeaccesspb.AccessScheme, msg proto.Message) {
	core.Access = &schemeaccesspb.AccessScheme_DatabaseCassandra{DatabaseCassandra: msg.(*schemeaccesspb.AccessScheme_Database_Cassandra)}
}

func NewDatabaseSQLServer() proto.Message { return &schemeaccesspb.AccessScheme_Database_SQLServer{} }
func SetDatabaseSQLServer(core *schemeaccesspb.AccessScheme, msg proto.Message) {
	core.Access = &schemeaccesspb.AccessScheme_DatabaseSqlserver{DatabaseSqlserver: msg.(*schemeaccesspb.AccessScheme_Database_SQLServer)}
}

func NewDatabaseSnowflake() proto.Message { return &schemeaccesspb.AccessScheme_Database_Snowflake{} }
func SetDatabaseSnowflake(core *schemeaccesspb.AccessScheme, msg proto.Message) {
	core.Access = &schemeaccesspb.AccessScheme_DatabaseSnowflake{DatabaseSnowflake: msg.(*schemeaccesspb.AccessScheme_Database_Snowflake)}
}

func (s *handler) handleDatabaseMysql(in *schemeaccesspb.AccessScheme_DatabaseMysql) (*SchemeDescription, error) {
	schemeType := schemeaccesspb.SchemeType_database_mysql
	newAccess := in.DatabaseMysql
	return &SchemeDescription{
		SchemeType:   schemeType,
		Access:       newAccess,
		TitlePattern: "Database MySQL #%s",
	}, nil
}

func (s *handler) handleDatabasePostgres(in *schemeaccesspb.AccessScheme_DatabasePostgres) (*SchemeDescription, error) {
	schemeType := schemeaccesspb.SchemeType_database_postgres
	newAccess := in.DatabasePostgres
	return &SchemeDescription{
		SchemeType:   schemeType,
		Access:       newAccess,
		TitlePattern: "Database PostgresQL #%s",
	}, nil
}

func (s *handler) handleDatabaseRedis(in *schemeaccesspb.AccessScheme_DatabaseRedis) (*SchemeDescription, error) {
	schemeType := schemeaccesspb.SchemeType_database_redis
	newAccess := in.DatabaseRedis
	return &SchemeDescription{
		SchemeType:   schemeType,
		Access:       newAccess,
		TitlePattern: "Database Redis #%s",
	}, nil
}

func (s *handler) handleDatabaseMongodb(in *schemeaccesspb.AccessScheme_DatabaseMongodb) (*SchemeDescription, error) {
	schemeType := schemeaccesspb.SchemeType_database_mongodb
	newAccess := in.DatabaseMongodb
	return &SchemeDescription{
		SchemeType:   schemeType,
		Access:       newAccess,
		TitlePattern: "Database MongoDB #%s",
	}, nil
}

func (s *handler) handleDatabaseElastic(in *schemeaccesspb.AccessScheme_DatabaseElastic) (*SchemeDescription, error) {
	schemeType := schemeaccesspb.SchemeType_database_elastic
	newAccess := in.DatabaseElastic
	return &SchemeDescription{
		SchemeType:   schemeType,
		Access:       newAccess,
		TitlePattern: "Database Elastic #%s",
	}, nil
}

func (s *handler) handleDatabaseDynamodb(in *schemeaccesspb.AccessScheme_DatabaseDynamodb) (*SchemeDescription, error) {
	schemeType := schemeaccesspb.SchemeType_database_dynamodb
	newAccess := in.DatabaseDynamodb
	return &SchemeDescription{
		SchemeType:   schemeType,
		Access:       newAccess,
		TitlePattern: "Database DynamoDB #%s",
	}, nil
}

func (s *handler) handleDatabaseCassandra(in *schemeaccesspb.AccessScheme_DatabaseCassandra) (*SchemeDescription, error) {
	schemeType := schemeaccesspb.SchemeType_database_cassandra
	newAccess := in.DatabaseCassandra
	return &SchemeDescription{
		SchemeType:   schemeType,
		Access:       newAccess,
		TitlePattern: "Database Cassandra #%s",
	}, nil
}

func (s *handler) handleDatabaseSqlserver(in *schemeaccesspb.AccessScheme_DatabaseSqlserver) (*SchemeDescription, error) {
	schemeType := schemeaccesspb.SchemeType_database_sqlserver
	newAccess := in.DatabaseSqlserver
	return &SchemeDescription{
		SchemeType:   schemeType,
		Access:       newAccess,
		TitlePattern: "Database Microsoft SQL Server #%s",
	}, nil
}

func (s *handler) handleDatabaseSnowflake(in *schemeaccesspb.AccessScheme_DatabaseSnowflake) (*SchemeDescription, error) {
	schemeType := schemeaccesspb.SchemeType_database_snowflake
	newAccess := in.DatabaseSnowflake
	return &SchemeDescription{
		SchemeType:   schemeType,
		Access:       newAccess,
		TitlePattern: "Database Snowflake #%s",
	}, nil
}
