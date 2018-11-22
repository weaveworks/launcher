package weavecloud

import (
	"bytes"
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
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return "", "", fmt.Errorf("Invalid token")
	} else if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf(resp.Status)
	}

	instance := lookupInstanceByTokenView{}
	err = json.NewDecoder(resp.Body).Decode(&instance)
	if err != nil {
		return "", "", err
	}

	return instance.ExternalID, instance.Name, nil
}

type platformVersionUpdate struct {
	PlatformVersion string `json:"platformVersion"`
}

// DefaultWCOrgPlatformVersionURLTemplate is the default URL template for UpdateInstancePlatformVersionByToken
const DefaultWCOrgPlatformVersionURLTemplate = "https://{{.WCHostname}}/api/users/org/platform_version"

// UpdateInstancePlatformVersionByToken updates the instance platform version given an instance token
func UpdateInstancePlatformVersionByToken(apiURL, token, platformVersion string) error {
	buf := &bytes.Buffer{}
	platformInfo := platformVersionUpdate{PlatformVersion: platformVersion}
	if err := json.NewEncoder(buf).Encode(platformInfo); err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPut, apiURL, buf)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("Invalid token")
	} else if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf(resp.Status)
	}

	return nil
}
