package weavecloud

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type lookupInstanceByTokenView struct {
	Name       string `json:"name"`
	ExternalID string `json:"externalID"`
}

// DefaultWCOrgLookupURLTemplate is the default URL template for LookupInstanceByToken
const DefaultWCOrgLookupURLTemplate = "https://{{.WCHostname}}/api/users/org/lookup"

// LookupInstanceByToken returns the instance ID given an instance token
func LookupInstanceByToken(apiURL, token string) (string, string, error) {
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}

	instance := lookupInstanceByTokenView{}

	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&instance)
	if err != nil {
		return "", "", err
	}

	return instance.ExternalID, instance.Name, nil
}
