package event

import (
	"testing"
	"time"

	"github.com/werbot/werbot/api/auth"
	"github.com/werbot/werbot/internal/utils/test"
	"github.com/werbot/werbot/internal/utils/test/factory"
	"github.com/werbot/werbot/pkg/uuid"
)

const (
	pathEvent        = "/v1/event"
	pathEventProfile = pathEvent + "/profile"
	pathEventProject = pathEvent + "/project"
	pathEventScheme  = pathEvent + "/scheme"

	adminPassword = "admin@werbot.net"
	userPassword  = "user@werbot.net"

	schemeEventCount = 11
)

// Known session values owned by the created graph. Searches assert these
// values so a match cannot come from unrelated rows.
const (
	adminProfileUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.9; rv:35.0) Gecko/20100101 Firefox/35."
	adminProfileIP        = "2001:0db8:85a3:0000:0000:8a2e:0370:7334"
	userProfileUserAgent  = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_2_0) Gecko/20100101 Firefox/68.0"
	userProfileIP         = "192.168.120.22"
	projectAndroidAgent   = "Mozilla/5.0 (Linux; U; Android 4.0.4; en-us; KFJWI Build/IMM76D) AppleWebKit/537.36 (KHTML, like Gecko) Silk/3.68 like Chrome/39.0.2171.93 Safari/537.36"
	projectAndroidIP      = "192.168.0.1"
	projectFirefoxAgent   = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.10; rv:39.0) Gecko/20100101 Firefox/39.0"
	projectIPv6IP         = "2001:0db8:beef:0000:0000:8a2e:0370:7334"
	schemeFirstUserAgent  = "Mozilla/5.0 (Windows NT 6.3; WOW64; Trident/7.0; Touch; LCJB; rv:11.0) like Gecko"
	schemeFirstIP         = "192.168.1.1"
)

// eventSession is the known session data of a created event.
type eventSession struct {
	ID, UserAgent, IP string
}

// eventEnv holds the fixture-free profile, project and scheme event graph
// created by setupTest.
type eventEnv struct {
	admin, user             factory.Profile
	adminHeader, userHeader map[string]string

	project factory.Project
	scheme  factory.Scheme

	adminProfileEvents []factory.Event
	adminProfileEvent  factory.Event
	adminProfileSess   eventSession

	userProfileEvents []factory.Event
	userProfileEvent  factory.Event
	userProfileSess   eventSession

	projectEvents   []factory.Event
	projectAndroid  factory.Event
	projectAndroidS eventSession
	projectIPv6     factory.Event
	projectIPv6S    eventSession

	schemeEvents []factory.Event
	schemeFirst  factory.Event
	schemeFirstS eventSession
}

func setupTest(t *testing.T) (*test.APIHandler, *eventEnv, func(t *testing.T)) {
	app, teardownTestCase := test.APIWithoutFixtures(t)
	auth.New(app.Handler).Routes()
	New(app.Handler).Routes()
	app.AddRoute404()

	builder := factory.New(t, app.Postgres.Conn().Conn)

	env := &eventEnv{}
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

	env.project = builder.Project(factory.ProjectParams{
		OwnerID: env.admin.ID,
		Title:   "event project",
		Alias:   "Event01",
	})
	env.scheme = builder.Scheme(factory.SchemeParams{
		ProjectID: env.project.ID,
		Title:     "event scheme",
	})

	// Profile events. Two admin events keep the limit=1 pagination case
	// meaningful; one user event is enough for the owner-filter case.
	env.adminProfileSess = eventSession{
		ID:        uuid.New(),
		UserAgent: adminProfileUserAgent,
		IP:        adminProfileIP,
	}
	env.adminProfileEvent = builder.Event(factory.EventParams{
		RelatedID:    env.admin.ID,
		PrimeSection: 1,
		Section:      1,
		Type:         9,
		SessionID:    env.adminProfileSess.ID,
		UserAgent:    env.adminProfileSess.UserAgent,
		IP:           env.adminProfileSess.IP,
	})
	env.adminProfileEvents = append(env.adminProfileEvents,
		env.adminProfileEvent,
		builder.Event(factory.EventParams{
			RelatedID:    env.admin.ID,
			PrimeSection: 1,
			Section:      1,
			Type:         10,
		}),
	)

	env.userProfileSess = eventSession{
		ID:        uuid.New(),
		UserAgent: userProfileUserAgent,
		IP:        userProfileIP,
	}
	env.userProfileEvent = builder.Event(factory.EventParams{
		RelatedID:    env.user.ID,
		PrimeSection: 1,
		Section:      1,
		Type:         9,
		SessionID:    env.userProfileSess.ID,
		UserAgent:    env.userProfileSess.UserAgent,
		IP:           env.userProfileSess.IP,
	})
	env.userProfileEvents = append(env.userProfileEvents, env.userProfileEvent)

	// Project events: one type 1 Android/IPv4 event and one type 2 IPv6
	// event. Explicit timestamps make the newest row deterministic even
	// though the assertions below do not depend on row order.
	projectBase := time.Now().Add(-time.Hour)
	env.projectAndroidS = eventSession{
		ID:        uuid.New(),
		UserAgent: projectAndroidAgent,
		IP:        projectAndroidIP,
	}
	env.projectAndroid = builder.Event(factory.EventParams{
		RelatedID:    env.project.ID,
		PrimeSection: 2,
		Section:      1,
		Type:         1,
		SessionID:    env.projectAndroidS.ID,
		UserAgent:    env.projectAndroidS.UserAgent,
		IP:           env.projectAndroidS.IP,
		CreatedAt:    factory.Time(projectBase.Add(time.Minute)),
	})
	env.projectIPv6S = eventSession{
		ID:        uuid.New(),
		UserAgent: projectFirefoxAgent,
		IP:        projectIPv6IP,
	}
	env.projectIPv6 = builder.Event(factory.EventParams{
		RelatedID:    env.project.ID,
		PrimeSection: 2,
		Section:      1,
		Type:         2,
		SessionID:    env.projectIPv6S.ID,
		UserAgent:    env.projectIPv6S.UserAgent,
		IP:           env.projectIPv6S.IP,
		CreatedAt:    factory.Time(projectBase),
	})
	env.projectEvents = append(env.projectEvents, env.projectAndroid, env.projectIPv6)

	// Scheme events: eleven rows. The newest one is the known first event
	// read by the list and detail assertions.
	schemeBase := time.Now().Add(-time.Hour)
	for i := 0; i < schemeEventCount-1; i++ {
		event := builder.Event(factory.EventParams{
			RelatedID:    env.scheme.ID,
			PrimeSection: 3,
			Section:      1,
			Type:         int16(i + 2),
			CreatedAt:    factory.Time(schemeBase.Add(time.Duration(i) * time.Minute)),
		})
		env.schemeEvents = append(env.schemeEvents, event)
	}
	env.schemeFirstS = eventSession{
		ID:        uuid.New(),
		UserAgent: schemeFirstUserAgent,
		IP:        schemeFirstIP,
	}
	env.schemeFirst = builder.Event(factory.EventParams{
		RelatedID:    env.scheme.ID,
		PrimeSection: 3,
		Section:      1,
		Type:         1,
		SessionID:    env.schemeFirstS.ID,
		UserAgent:    env.schemeFirstS.UserAgent,
		IP:           env.schemeFirstS.IP,
		CreatedAt:    factory.Time(schemeBase.Add(schemeEventCount * time.Minute)),
	})
	env.schemeEvents = append(env.schemeEvents, env.schemeFirst)

	env.adminHeader = app.AuthHeader(env.admin.Email, adminPassword)
	env.userHeader = app.AuthHeader(env.user.Email, userPassword)

	return app, env, teardownTestCase
}
