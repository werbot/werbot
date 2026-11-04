package access

import (
	"testing"

	schemeaccesspb "github.com/werbot/werbot/internal/core/scheme/proto/access"
)

func TestUnmarshalNumericBools(t *testing.T) {
	tests := []struct {
		name       string
		schemeType schemeaccesspb.SchemeType
		payload    string
		check      func(t *testing.T, scheme *DecodedScheme)
	}{
		{
			name:       "redis tls_required 0",
			schemeType: schemeaccesspb.SchemeType_database_redis,
			payload:    `{"port":6379,"alias":"A9ya0Y","access":{"login":"base1","password":"password1"},"address":"192.22.138.1","tls_required":0}`,
			check: func(t *testing.T, scheme *DecodedScheme) {
				redis := scheme.GetDatabaseRedis()
				if redis == nil || redis.TlsRequired {
					t.Fatalf("redis access not decoded: %+v", redis)
				}
			},
		},
		{
			name:       "mongodb numeric bools",
			schemeType: schemeaccesspb.SchemeType_database_mongodb,
			payload:    `{"port":27017,"alias":"BH02hb","tls_required":1,"replica_connect":0}`,
			check: func(t *testing.T, scheme *DecodedScheme) {
				mongo := scheme.GetDatabaseMongodb()
				if mongo == nil || !mongo.TlsRequired || mongo.ReplicaConnect {
					t.Fatalf("mongodb access not decoded: %+v", mongo)
				}
			},
		},
		{
			name:       "sqlserver default_database 1",
			schemeType: schemeaccesspb.SchemeType_database_sqlserver,
			payload:    `{"port":1433,"default_database":1}`,
			check: func(t *testing.T, scheme *DecodedScheme) {
				sqlserver := scheme.GetDatabaseSqlserver()
				if sqlserver == nil || !sqlserver.DefaultDatabase {
					t.Fatalf("sqlserver access not decoded: %+v", sqlserver)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme, err := Unmarshal([]byte(tt.payload), tt.schemeType)
			if err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			tt.check(t, scheme)
		})
	}
}
