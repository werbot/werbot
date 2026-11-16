package policy

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/cedar-policy/cedar-go"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEntitiesJSONDeterministic(t *testing.T) {
	s := Snapshot{}
	for i := range 50 {
		s.Resources = append(s.Resources, SnapshotResource{ID: uuid.New(), Name: fmt.Sprintf("r%02d", i), Type: "ssh",
			Labels: map[string]string{"env": "prod", "team": fmt.Sprint(i % 3)}})
	}
	ps, _, err := Compile(s)
	require.NoError(t, err)

	first, err := ps.EntitiesJSON()
	require.NoError(t, err)
	var em cedar.EntityMap
	require.NoError(t, json.Unmarshal(first, &em))
	uid := cedar.NewEntityUID("Werbot::Resource", cedar.String(s.Resources[7].ID.String()))
	assert.Contains(t, em, uid)
	assert.Len(t, em, 50)

	second, err := ps.EntitiesJSON()
	require.NoError(t, err)
	assert.Equal(t, first, second)
}
