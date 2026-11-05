package scheme

import (
	"context"
	"fmt"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/werbot/werbot/api/auth"
	keyenum "github.com/werbot/werbot/internal/core/key/proto/enum"
	keymessage "github.com/werbot/werbot/internal/core/key/proto/message"
	"github.com/werbot/werbot/internal/utils/test"
	"github.com/werbot/werbot/internal/utils/test/factory"
	"github.com/werbot/werbot/pkg/crypto"
	"github.com/werbot/werbot/pkg/storage/redis"
)

const (
	pathSchemes = "/v1/schemes"

	adminPassword = "admin@werbot.net"
	userPassword  = "user@werbot.net"
)

// schemeEnv holds the fixture-free scheme graph created by setupTest.
type schemeEnv struct {
	b *factory.Builder

	admin, user             factory.Profile
	adminHeader, userHeader map[string]string

	project1, project3 factory.Project

	adminScheme1, adminScheme2, adminEventScheme factory.Scheme
	userScheme1, userScheme2                     factory.Scheme

	adminCountries []factory.SchemeFirewallCountry
	adminNetworks  []factory.SchemeFirewallNetwork
	userCountries  []factory.SchemeFirewallCountry
	userNetworks   []factory.SchemeFirewallNetwork
}

func setupTest(t *testing.T) (*test.APIHandler, *schemeEnv, func(t *testing.T)) {
	app, teardownTestCase := test.API(t)
	auth.New(app.Handler).Routes()
	New(app.Handler).Routes()
	app.AddRoute404()

	env := &schemeEnv{b: factory.New(t, app.Postgres.Conn().Conn)}

	env.admin = env.b.Profile(factory.ProfileParams{
		Alias:    "admin",
		Name:     "Penny",
		Surname:  "Hoyle",
		Email:    adminPassword,
		Password: adminPassword,
		Role:     3,
	})
	env.user = env.b.Profile(factory.ProfileParams{
		Alias:    "user",
		Name:     "Carly",
		Surname:  "Bender",
		Email:    userPassword,
		Password: userPassword,
		Role:     1,
	})

	env.project1 = env.b.Project(factory.ProjectParams{
		OwnerID: env.admin.ID,
		Title:   "project1",
		Alias:   "Y93iyI",
	})
	env.project3 = env.b.Project(factory.ProjectParams{
		OwnerID: env.user.ID,
		Title:   "project3",
		Alias:   "C8cXx0",
	})

	env.adminScheme1 = env.b.Scheme(factory.SchemeParams{
		ProjectID:  env.project1.ID,
		Title:      "Git server",
		SchemeType: 103,
		Active:     factory.Bool(false),
		Online:     factory.Bool(true),
		Access: map[string]any{
			"alias":   "onxzU5",
			"address": "secret.net.google.com",
			"port":    2206,
			"password": map[string]any{
				"login":    "ubuntu6",
				"password": "ubuntu6",
			},
		},
	})
	env.adminScheme2 = env.b.Scheme(factory.SchemeParams{
		ProjectID:  env.project1.ID,
		Title:      "CI/CD",
		SchemeType: 103,
		Audit:      factory.Bool(true),
		Online:     factory.Bool(true),
		Access: map[string]any{
			"alias":   "x8GlLZ",
			"address": "4f06:bbfc:a98e:7a58:b6de:93f8:4bcd:bcda",
			"port":    2205,
			"key": map[string]any{
				"login": "user3",
				"key": map[string]any{
					"public":     "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIC7L+QVM2RILf1C8PlTZ9Ab2aqW6/LrIBWiK4ybiKLzG",
					"private":    "private",
					"passphrase": "",
				},
			},
		},
	})
	env.adminEventScheme = env.b.Scheme(factory.SchemeParams{
		ProjectID:  env.project1.ID,
		Title:      "Develop server #2",
		SchemeType: 103,
		Access: map[string]any{
			"alias": "7Ub2Gn",
			"port":  2211,
			"key": map[string]any{
				"login": "ubuntu11",
				"key": map[string]any{
					"public":     "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIB1PFvdQd6ezh0gsz33IFVoK9Hn0TUqLTtCalrN0hyuO",
					"private":    "private",
					"passphrase": "test",
				},
			},
		},
	})
	env.userScheme1 = env.b.Scheme(factory.SchemeParams{
		ProjectID:  env.project3.ID,
		Title:      "Storage server backup",
		SchemeType: 103,
		Access: map[string]any{
			"alias":   "JiepgT",
			"address": "127.0.0.4",
			"port":    2207,
			"password": map[string]any{
				"login":    "test",
				"password": "test",
			},
		},
	})
	env.userScheme2 = env.b.Scheme(factory.SchemeParams{
		ProjectID:  env.project3.ID,
		Title:      "Develop server #3",
		SchemeType: 103,
		Access: map[string]any{
			"alias":   "IUkc7f",
			"address": "127.0.0.3",
			"port":    22,
			"key": map[string]any{
				"login": "user",
				"key": map[string]any{
					"public":     "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIB0JO0wPjURoFQUZOSI+ZH4l1gtd98bcFqbZCmSlHeBd",
					"private":    "private",
					"passphrase": "",
				},
			},
		},
	})

	env.adminHeader = app.AuthHeader(env.admin.Email, adminPassword)
	env.userHeader = app.AuthHeader(env.user.Email, userPassword)

	return app, env, teardownTestCase
}

// addGroups creates one scheme for every group except servers, so list totals
// cover all scheme type groups.
func (e *schemeEnv) addGroups(t *testing.T) {
	t.Helper()

	groups := []struct {
		title      string
		schemeType int16
		access     map[string]any
	}{
		{
			title:      "database",
			schemeType: 201,
			access: map[string]any{
				"alias":         "dbAlias",
				"address":       "192.151.43.1",
				"port":          3306,
				"database_name": "database1",
				"access":        map[string]any{"login": "base1", "password": "password1"},
				"mtls":          map[string]any{"server_ca": "", "client_cert": "", "client_key": ""},
			},
		},
		{
			title:      "desktop",
			schemeType: 301,
			access: map[string]any{
				"alias":   "rdpAlias",
				"address": "192.38.195.1",
				"port":    3389,
				"access":  map[string]any{"login": "login1", "password": "password1"},
			},
		},
		{
			title:      "container",
			schemeType: 401,
			access: map[string]any{
				"alias":   "dockerAlias",
				"address": "192.66.132.1",
				"port":    2375,
				"mtls":    map[string]any{"server_ca": "", "client_cert": "", "client_key": ""},
			},
		},
		{
			title:      "cloud",
			schemeType: 501,
			access: map[string]any{
				"alias":  "cloudAlias",
				"access": map[string]any{"access_key_id": "", "secret_access_key": "", "role_arn": "", "role_external_id": ""},
			},
		},
		{
			title:      "application",
			schemeType: 601,
			access:     map[string]any{"alias": "siteAlias"},
		},
	}

	for _, group := range groups {
		e.b.Scheme(factory.SchemeParams{
			ProjectID:  e.project1.ID,
			Title:      group.title,
			SchemeType: group.schemeType,
			Access:     group.access,
		})
	}
}

// addFirewall creates the scheme firewall rows used by the firewall cases.
func (e *schemeEnv) addFirewall(t *testing.T) {
	t.Helper()

	for _, code := range []string{"CN", "RU", "US"} {
		e.adminCountries = append(e.adminCountries, e.b.SchemeFirewallCountry(factory.SchemeFirewallCountryParams{
			SchemeID:    e.adminScheme1.ID,
			CountryCode: code,
		}))
	}
	for _, network := range []string{"192.168.1.0/24", "127.0.0.3/32"} {
		e.adminNetworks = append(e.adminNetworks, e.b.SchemeFirewallNetwork(factory.SchemeFirewallNetworkParams{
			SchemeID: e.adminScheme1.ID,
			Network:  network,
		}))
	}
	for _, code := range []string{"BY", "LT", "PL", "LV"} {
		e.userCountries = append(e.userCountries, e.b.SchemeFirewallCountry(factory.SchemeFirewallCountryParams{
			SchemeID:    e.userScheme1.ID,
			CountryCode: code,
		}))
	}
	for _, network := range []string{"192.168.1.0/24", "192.168.33.1/32", "127.0.0.3/32"} {
		e.userNetworks = append(e.userNetworks, e.b.SchemeFirewallNetwork(factory.SchemeFirewallNetworkParams{
			SchemeID: e.userScheme1.ID,
			Network:  network,
		}))
	}
}

// addActivity applies the weekly activity grids the activity cases assert.
func (e *schemeEnv) addActivity(t *testing.T) {
	t.Helper()

	e.b.SchemeActivity(e.adminScheme1.ID, map[string][]int32{
		"mon": {0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
		"tue": {1, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 1, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0},
		"wed": {1, 1, 0, 1, 1, 1, 1, 1, 1, 1, 0, 1, 1, 1, 0, 1, 1, 1, 1, 1, 1, 1, 0, 1},
		"thu": {1, 1, 1, 0, 1, 1, 1, 1, 1, 0, 1, 1, 1, 1, 1, 0, 1, 1, 1, 1, 1, 0, 1, 1},
		"fri": {1, 1, 1, 1, 0, 1, 1, 1, 0, 1, 1, 1, 1, 1, 1, 1, 0, 1, 1, 1, 0, 1, 1, 1},
		"sat": {1, 1, 1, 1, 1, 0, 1, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 1, 0, 1, 1, 1, 1},
		"sun": {1, 1, 1, 1, 1, 1, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 1, 1, 1, 1, 1},
	})

	e.b.SchemeActivity(e.userScheme1.ID, map[string][]int32{
		"mon": {1, 1, 1, 1, 1, 1, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 1, 1, 1, 1, 1},
		"tue": {1, 1, 1, 1, 1, 0, 1, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 1, 0, 1, 1, 1, 1},
		"wed": {1, 1, 1, 1, 0, 1, 1, 1, 0, 1, 1, 1, 1, 1, 1, 1, 0, 1, 1, 1, 0, 1, 1, 1},
		"thu": {1, 1, 1, 0, 1, 1, 1, 1, 1, 0, 1, 1, 1, 1, 1, 0, 1, 1, 1, 1, 1, 0, 1, 1},
		"fri": {1, 1, 0, 1, 1, 1, 1, 1, 1, 1, 0, 1, 1, 1, 0, 1, 1, 1, 1, 1, 1, 1, 0, 1},
		"sat": {1, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 1, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0},
		"sun": {0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
	})
}

// addProfileSchemes creates the independent projects, members and schemes that
// the profileSchemes cases aggregate. It returns after linking every scheme
// member row.
func (e *schemeEnv) addProfileSchemes(t *testing.T) {
	t.Helper()

	adminProject := e.b.Project(factory.ProjectParams{OwnerID: e.admin.ID, Title: "profile admin", Alias: "profAD"})
	userProject := e.b.Project(factory.ProjectParams{OwnerID: e.user.ID, Title: "profile user", Alias: "profUS"})

	adminMember := e.b.ProjectMember(factory.ProjectMemberParams{ProjectID: adminProject.ID, ProfileID: e.admin.ID})
	userMember := e.b.ProjectMember(factory.ProjectMemberParams{ProjectID: userProject.ID, ProfileID: e.user.ID})

	link := func(projectID, memberID string, schemeType int16, alias string) {
		scheme := e.b.Scheme(factory.SchemeParams{
			ProjectID:  projectID,
			Title:      alias,
			SchemeType: schemeType,
			Access:     map[string]any{"alias": alias},
		})
		e.b.SchemeMember(factory.SchemeMemberParams{
			SchemeID:        scheme.ID,
			ProjectMemberID: memberID,
		})
	}

	schemeTypes := []struct {
		schemeType int16
		alias      string
	}{
		{101, "server"},
		{201, "database"},
		{301, "desktop-1"},
		{302, "desktop-2"},
		{401, "container"},
		{501, "cloud"},
		{601, "application"},
	}
	for _, scheme := range schemeTypes {
		link(adminProject.ID, adminMember.ID, scheme.schemeType, "admin-"+scheme.alias)
		link(userProject.ID, userMember.ID, scheme.schemeType, "user-"+scheme.alias)
	}

	// The same cloud scheme is shared through the same project member twice.
	// Totals count both rows even though the list returns one row per row.
	cloud := e.b.Scheme(factory.SchemeParams{
		ProjectID:  userProject.ID,
		Title:      "user-cloud-duplicate",
		SchemeType: 501,
		Access:     map[string]any{"alias": "user-cloud-duplicate"},
	})
	e.b.SchemeMember(factory.SchemeMemberParams{SchemeID: cloud.ID, ProjectMemberID: userMember.ID})
	e.b.SchemeMember(factory.SchemeMemberParams{SchemeID: cloud.ID, ProjectMemberID: userMember.ID})
}

func serverKeygen(rd *redis.Connect, uuid string, empty bool) func() {
	return func() {
		cacheKeyStr := fmt.Sprintf("tmp_key_ssh:%s", uuid)
		expiration := time.Duration(10 * float64(time.Second))

		if empty {
			_ = rd.Client.Set(context.Background(), cacheKeyStr, "{}", expiration)
		} else {
			newKeySSH, _ := crypto.NewSSHKey(keyenum.KeyType_ed25519.String())
			schemeKey := &keymessage.SchemeKey{
				Public:      string(newKeySSH.PublicKey),
				Private:     string(newKeySSH.PrivateKey),
				Passphrase:  newKeySSH.Passphrase,
				FingerPrint: newKeySSH.FingerPrint,
			}
			json, _ := protojson.Marshal(schemeKey)
			_ = rd.Client.Set(context.Background(), cacheKeyStr, string(json), expiration)
		}
	}
}
