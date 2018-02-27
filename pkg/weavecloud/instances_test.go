package weavecloud

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	instanceName  = "Awesome Instance"
	instanceID    = "awesome-instance"
	instanceToken = "WEAVE_CLOUD_TOKEN_123"
)

func TestLookupInstanceByToken(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		assert.Equal(t, fmt.Sprintf("Bearer %s", instanceToken), authHeader)

		json.NewEncoder(w).Encode(
			lookupInstanceByTokenView{
				ExternalID: instanceID,
				Name:       instanceName,
			},
		)
	}))
	defer ts.Close()

	id, name, err := LookupInstanceByToken(ts.URL, instanceToken)
	assert.NoError(t, err)
	assert.Equal(t, instanceID, id)
	assert.Equal(t, instanceName, name)
}
