package access

import (
	"context"
	"database/sql"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	schemeaccesspb "github.com/werbot/werbot/internal/grpc/scheme/proto/access"
	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/pkg/crypto"
	"github.com/werbot/werbot/pkg/logger"
	"github.com/werbot/werbot/pkg/storage/postgres"
	"github.com/werbot/werbot/pkg/storage/redis"
)

// SchemeHandler is ...
type SchemeHandler struct {
	DB           *postgres.Connect
	Redis        *redis.Connect
	AccessScheme *schemeaccesspb.AccessScheme
	Update       *OwnerScheme
	Log          logger.Logger
}

// OwnerScheme is ...
type OwnerScheme struct {
	SchemeID  string
	ProjectID string
	OwnerID   string
}

// SchemeDescription  is ...
type SchemeDescription struct {
	Scheme       any
	SchemeType   schemeaccesspb.SchemeType
	Access       protoreflect.ProtoMessage
	TitlePattern string
}

type handler struct {
	sh         SchemeHandler
	alias      string
	schemeType schemeaccesspb.SchemeType
	schemeByte []byte
}

// Scheme is ...
func Scheme(ctx context.Context, sh *SchemeHandler) (*SchemeDescription, error) {
	handler := handler{
		sh: *sh,
	}

	// update mode --------------------
	if sh.Update != nil {
		// define scheme type for schemeID
		err := sh.DB.Conn.QueryRowContext(ctx, `
      SELECT
        "scheme_type",
        "scheme"."access"
      FROM "scheme"
        INNER JOIN "project" ON "scheme"."project_id" = "project"."id"
      WHERE
        "scheme"."project_id" = "project"."id"
        AND "scheme"."id" = $1
        AND "project"."id" = $2
        AND "project"."owner_id" = $3
    `,
			sh.Update.SchemeID,
			sh.Update.ProjectID,
			sh.Update.OwnerID,
		).Scan(
			&handler.schemeType,
			&handler.schemeByte,
		)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, trace.Error(err, sh.Log, nil)
		}
	} else {
		handler.alias = crypto.NewPassword(6, false)
	}
	// end update mode --------------------

	switch newAccess := sh.AccessScheme.GetAccess().(type) {
	// server section
	case *schemeaccesspb.AccessScheme_ServerTcp:
		return handler.handleServerTCP(newAccess)
	case *schemeaccesspb.AccessScheme_ServerUdp:
		return handler.handleServerUDP(newAccess)
	case *schemeaccesspb.AccessScheme_ServerSsh:
		return handler.handleServerSSH(newAccess)
	case *schemeaccesspb.AccessScheme_ServerTelnet:
		return handler.handleServerTelnet(newAccess)

	// database section
	case *schemeaccesspb.AccessScheme_DatabaseMysql:
		return handler.handleDatabaseMysql(newAccess)
	case *schemeaccesspb.AccessScheme_DatabasePostgres:
		return handler.handleDatabasePostgres(newAccess)
	case *schemeaccesspb.AccessScheme_DatabaseRedis:
		return handler.handleDatabaseRedis(newAccess)
	case *schemeaccesspb.AccessScheme_DatabaseMongodb:
		return handler.handleDatabaseMongodb(newAccess)
	case *schemeaccesspb.AccessScheme_DatabaseElastic:
		return handler.handleDatabaseElastic(newAccess)
	case *schemeaccesspb.AccessScheme_DatabaseDynamodb:
		return handler.handleDatabaseDynamodb(newAccess)
	case *schemeaccesspb.AccessScheme_DatabaseCassandra:
		return handler.handleDatabaseCassandra(newAccess)
	case *schemeaccesspb.AccessScheme_DatabaseSqlserver:
		return handler.handleDatabaseSqlserver(newAccess)
	case *schemeaccesspb.AccessScheme_DatabaseSnowflake:
		return handler.handleDatabaseSnowflake(newAccess)

	// desktop section
	case *schemeaccesspb.AccessScheme_DesktopRdp:
		return handler.handleDesktopRDP(newAccess)
	case *schemeaccesspb.AccessScheme_DesktopVnc:
		return handler.handleDesktopVNC(newAccess)

	// container section
	case *schemeaccesspb.AccessScheme_ContainerDocker:
		return handler.handleContainerDocker(newAccess)
	case *schemeaccesspb.AccessScheme_ContainerK8S:
		return handler.handleContainerK8S(newAccess)

	// cloud section
	case *schemeaccesspb.AccessScheme_CloudAws:
		return handler.handleCloudAWS(newAccess)
	case *schemeaccesspb.AccessScheme_CloudGcp:
		return handler.handleCloudGCP(newAccess)
	case *schemeaccesspb.AccessScheme_CloudAzure:
		return handler.handleCloudAzure(newAccess)
	case *schemeaccesspb.AccessScheme_CloudDo:
		return handler.handleCloudDO(newAccess)
	case *schemeaccesspb.AccessScheme_CloudHetzner:
		return handler.handleCloudHetzner(newAccess)

	// application section
	case *schemeaccesspb.AccessScheme_ApplicationSite:
		return handler.handleApplicationSite(newAccess)
	}

	errGRPC := status.Error(codes.NotFound, trace.MsgSchemeNotFound)
	return nil, trace.Error(errGRPC, sh.Log, nil)
}

// -------
type UnmarshalTypeMap struct {
	NewInstance func() proto.Message
	SetAccess   func(*schemeaccesspb.AccessScheme, proto.Message)
}

func TypeToScheme(st schemeaccesspb.SchemeType) UnmarshalTypeMap {
	unmarshalTypeMap := map[schemeaccesspb.SchemeType]UnmarshalTypeMap{
		// server section
		schemeaccesspb.SchemeType_server_tcp:    {NewInstance: NewServerTCP, SetAccess: SetServerTCP},
		schemeaccesspb.SchemeType_server_udp:    {NewInstance: NewServerUDP, SetAccess: SetServerUDP},
		schemeaccesspb.SchemeType_server_ssh:    {NewInstance: NewServerSSH, SetAccess: SetServerSSH},
		schemeaccesspb.SchemeType_server_telnet: {NewInstance: NewServerTelnet, SetAccess: SetServerTelnet},

		// database section
		schemeaccesspb.SchemeType_database_mysql:     {NewInstance: NewDatabaseMySQL, SetAccess: SetDatabaseMySQL},
		schemeaccesspb.SchemeType_database_postgres:  {NewInstance: NewDatabasePostgres, SetAccess: SetDatabasePostgres},
		schemeaccesspb.SchemeType_database_redis:     {NewInstance: NewDatabaseRedis, SetAccess: SetDatabaseRedis},
		schemeaccesspb.SchemeType_database_mongodb:   {NewInstance: NewDatabaseMongoDB, SetAccess: SetDatabaseMongoDB},
		schemeaccesspb.SchemeType_database_elastic:   {NewInstance: NewDatabaseElastic, SetAccess: SetDatabaseElastic},
		schemeaccesspb.SchemeType_database_dynamodb:  {NewInstance: NewDatabaseDynamoDB, SetAccess: SetDatabaseDynamoDB},
		schemeaccesspb.SchemeType_database_cassandra: {NewInstance: NewDatabaseCassandra, SetAccess: SetDatabaseCassandra},
		schemeaccesspb.SchemeType_database_sqlserver: {NewInstance: NewDatabaseSQLServer, SetAccess: SetDatabaseSQLServer},
		schemeaccesspb.SchemeType_database_snowflake: {NewInstance: NewDatabaseSnowflake, SetAccess: SetDatabaseSnowflake},

		// desktop section
		schemeaccesspb.SchemeType_desktop_rdp: {NewInstance: NewDesktopRDP, SetAccess: SetDesktopRDP},
		schemeaccesspb.SchemeType_desktop_vnc: {NewInstance: NewDesktopVNC, SetAccess: SetDesktopVNC},

		// container section
		schemeaccesspb.SchemeType_container_docker: {NewInstance: NewContainerDocker, SetAccess: SetContainerDocker},
		schemeaccesspb.SchemeType_container_k8s:    {NewInstance: NewContainerK8S, SetAccess: SetContainerK8S},

		// cloud section
		schemeaccesspb.SchemeType_cloud_aws:     {NewInstance: NewCloudAWS, SetAccess: SetCloudAWS},
		schemeaccesspb.SchemeType_cloud_gcp:     {NewInstance: NewCloudGCP, SetAccess: SetCloudGCP},
		schemeaccesspb.SchemeType_cloud_azure:   {NewInstance: NewCloudAzure, SetAccess: SetCloudAzure},
		schemeaccesspb.SchemeType_cloud_do:      {NewInstance: NewCloudDO, SetAccess: SetCloudDO},
		schemeaccesspb.SchemeType_cloud_hetzner: {NewInstance: NewCloudHetzner, SetAccess: SetCloudHetzner},

		// application section
		schemeaccesspb.SchemeType_application_site: {NewInstance: NewApplicationSite, SetAccess: SetApplicationSite},
	}

	if val, ok := unmarshalTypeMap[st]; ok {
		return val
	}

	return UnmarshalTypeMap{}
}
