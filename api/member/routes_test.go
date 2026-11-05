package member

import (
	"fmt"
	"testing"
	"time"

	"github.com/werbot/werbot/api/auth"
	"github.com/werbot/werbot/internal/utils/test"
	"github.com/werbot/werbot/internal/utils/test/factory"
)

const (
	pathMembers              = "/v1/members"
	pathMembersProject       = pathMembers + "/project"
	pathMembersScheme        = pathMembers + "/scheme"
	pathMembersInvite        = pathMembers + "/invite"
	pathMembersInviteProject = pathMembersInvite + "/project"

	adminPassword = "admin@werbot.net"
	userPassword  = "user@werbot.net"
)

// memberEnv holds the fixture-free member, project and scheme graph created by
// setupTest.
type memberEnv struct {
	admin, user             factory.Profile
	invite201, user20       factory.Profile
	profiles                map[string]factory.Profile
	adminHeader, userHeader map[string]string

	project1, project3 factory.Project

	adminScheme1, adminScheme2 factory.Scheme
	userScheme1, userScheme2   factory.Scheme

	p1Members, p3Members map[string]factory.ProjectMember

	adminScheme1User factory.SchemeMember
	userScheme1Admin factory.SchemeMember
	userScheme2User1 factory.SchemeMember
}

func setupTest(t *testing.T) (*test.APIHandler, *memberEnv, func(t *testing.T)) {
	app, teardownTestCase := test.API(t)
	auth.New(app.Handler).Routes()
	New(app.Handler).Routes()
	app.AddRoute404()

	builder := factory.New(t, app.Postgres.Conn().Conn)

	env := &memberEnv{
		profiles:  map[string]factory.Profile{},
		p1Members: map[string]factory.ProjectMember{},
		p3Members: map[string]factory.ProjectMember{},
	}

	env.admin = builder.Profile(factory.ProfileParams{
		Alias:    "admin",
		Name:     "Penny",
		Surname:  "Hoyle",
		Email:    adminPassword,
		Password: adminPassword,
		Role:     3,
	})
	env.user = builder.Profile(factory.ProfileParams{
		Alias:    "user",
		Name:     "Carly",
		Surname:  "Bender",
		Email:    userPassword,
		Password: userPassword,
		Role:     1,
	})
	env.profiles["admin"] = env.admin
	env.profiles["user"] = env.user

	people := []struct {
		alias, name, surname string
	}{
		{"user1", "Harrison", "Bowling"},
		{"user2", "Terry", "Henry"},
		{"user3", "Carla", "Snyder"},
		{"user4", "Clint", "McLean"},
		{"user5", "Chris", "Burch"},
		{"user6", "Justine", "Wolfson"},
		{"user7", "Lori", "Henry"},
		{"user8", "Kendrick", "Hall"},
		{"user9", "Brock", "Solomon"},
		{"user10", "Clinton", "Proctor"},
		{"user11", "Noel", "Terrell"},
		{"invite201", "invite201", "invite201"},
		{"user20", "Kyleigh", "Steele"},
	}
	for _, person := range people {
		profile := builder.Profile(factory.ProfileParams{
			Alias:   person.alias,
			Name:    person.name,
			Surname: person.surname,
			Email:   fmt.Sprintf("%s@werbot.net", person.alias),
		})
		env.profiles[person.alias] = profile
	}

	env.invite201 = env.profiles["invite201"]
	env.user20 = env.profiles["user20"]

	env.project1 = builder.Project(factory.ProjectParams{
		OwnerID: env.admin.ID,
		Title:   "project1",
		Alias:   "Y93iyI",
	})
	env.project3 = builder.Project(factory.ProjectParams{
		OwnerID: env.user.ID,
		Title:   "project3",
		Alias:   "C8cXx0",
	})

	// created_at is the list sort key. Create the last entry first so the
	// expected row order stays deterministic.
	p1Flags := map[string]struct{ active, online bool }{
		"user":  {true, false},
		"user1": {true, true},
		"user2": {true, true},
		"user3": {false, false},
		"user4": {true, true},
		"user5": {true, false},
		"user6": {false, false},
	}
	p1Order := []string{"user", "user1", "user2", "user4", "user5", "user6", "user3"}
	p1Base := time.Now().Add(-time.Hour)
	for i, alias := range p1Order {
		flags := p1Flags[alias]
		env.p1Members[alias] = builder.ProjectMember(factory.ProjectMemberParams{
			ProjectID: env.project1.ID,
			ProfileID: env.profiles[alias].ID,
			Active:    factory.Bool(flags.active),
			Online:    factory.Bool(flags.online),
			CreatedAt: factory.Time(p1Base.Add(time.Duration(i) * time.Minute)),
		})
	}

	p3Order := []string{"user2", "user3", "user4", "user5", "user6", "user7", "user8", "user9", "user10", "user11", "admin", "user1"}
	p3Base := time.Now().Add(-2 * time.Hour)
	for i, alias := range p3Order {
		params := factory.ProjectMemberParams{
			ProjectID: env.project3.ID,
			ProfileID: env.profiles[alias].ID,
			Active:    factory.Bool(true),
			Online:    factory.Bool(true),
			CreatedAt: factory.Time(p3Base.Add(time.Duration(i) * time.Minute)),
		}
		if alias == "admin" {
			params.Online = factory.Bool(false)
		}
		if alias == "user2" {
			params.Active = factory.Bool(false)
			params.LockedAt = factory.Time(p3Base.Add(time.Hour))
		}
		env.p3Members[alias] = builder.ProjectMember(params)
	}

	env.adminScheme1 = builder.Scheme(factory.SchemeParams{ProjectID: env.project1.ID, Title: "admin scheme 1", SchemeType: 103})
	env.adminScheme2 = builder.Scheme(factory.SchemeParams{ProjectID: env.project1.ID, Title: "admin scheme 2"})
	env.userScheme1 = builder.Scheme(factory.SchemeParams{ProjectID: env.project3.ID, Title: "user scheme 1", SchemeType: 201})
	env.userScheme2 = builder.Scheme(factory.SchemeParams{ProjectID: env.project3.ID, Title: "user scheme 2", SchemeType: 301})

	env.adminScheme1User = builder.SchemeMember(factory.SchemeMemberParams{
		SchemeID:        env.adminScheme1.ID,
		ProjectMemberID: env.p1Members["user"].ID,
		Active:          factory.Bool(true),
	})
	env.userScheme1Admin = builder.SchemeMember(factory.SchemeMemberParams{
		SchemeID:        env.userScheme1.ID,
		ProjectMemberID: env.p3Members["admin"].ID,
		Active:          factory.Bool(true),
		Online:          factory.Bool(true),
		UpdatedAt:       factory.Time(time.Now()),
	})
	env.userScheme2User1 = builder.SchemeMember(factory.SchemeMemberParams{
		SchemeID:        env.userScheme2.ID,
		ProjectMemberID: env.p3Members["user1"].ID,
		Active:          factory.Bool(true),
	})
	builder.SchemeMember(factory.SchemeMemberParams{
		SchemeID:        env.userScheme2.ID,
		ProjectMemberID: env.p3Members["admin"].ID,
		Active:          factory.Bool(true),
	})

	env.adminHeader = app.AuthHeader(env.admin.Email, adminPassword)
	env.userHeader = app.AuthHeader(env.user.Email, userPassword)

	return app, env, teardownTestCase
}
