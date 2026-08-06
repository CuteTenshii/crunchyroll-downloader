package engine

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Multiprofile endpoints are best-effort. They are not part of a stable public
// contract; failures return an empty list rather than failing Home.
var (
	accountsMeURL = "https://www.crunchyroll.com/accounts/v1/me"
	// multiprofileURL builds GET /accounts/v1/{accountId}/multiprofile
	multiprofileURL = func(accountID string) string {
		return "https://www.crunchyroll.com/accounts/v1/" + url.PathEscape(accountID) + "/multiprofile"
	}
	// multiprofileActivateURL builds POST activate when documented shapes exist.
	multiprofileActivateURL = func(accountID, profileID string) string {
		return "https://www.crunchyroll.com/accounts/v1/" + url.PathEscape(accountID) +
			"/multiprofile/" + url.PathEscape(profileID) + "/activate"
	}
)

// ListCRProfiles fetches multiprofile entries for the authenticated account.
// Returns an empty slice (nil error) when the API is unavailable, the account
// has no multiprofile payload, or auth identity is missing.
// Requires a prior successful AuthenticateFromCookieFile / token refresh.
func ListCRProfiles() ([]CRProfile, error) {
	accountID := GetAccountID()
	if accountID == "" {
		// Attempt /me to recover identity; still best-effort.
		if profiles, ok := tryListFromMe(); ok {
			return profiles, nil
		}
		return []CRProfile{}, nil
	}

	if profiles, ok := tryListMultiprofile(accountID); ok {
		return profiles, nil
	}
	if profiles, ok := tryListFromMe(); ok {
		return profiles, nil
	}
	return []CRProfile{}, nil
}

// SwitchCRProfile attempts to activate a multiprofile on the provider when a
// switch endpoint exists. On API failure it returns nil so callers can still
// persist ActiveCRProfileID and re-auth. A hard error is only returned when
// profileID is empty.
func SwitchCRProfile(profileID string) error {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return fmt.Errorf("profile id is required")
	}
	accountID := GetAccountID()
	if accountID == "" {
		// Soft success: prefs-only switch; re-auth may recover identity later.
		return nil
	}

	// Try a few plausible activate shapes; ignore non-2xx / network errors.
	endpoints := []string{
		multiprofileActivateURL(accountID, profileID),
		"https://www.crunchyroll.com/accounts/v1/" + url.PathEscape(accountID) +
			"/multiprofile/" + url.PathEscape(profileID),
	}
	for _, endpoint := range endpoints {
		if tryActivateProfile(endpoint) {
			return nil
		}
	}
	return nil
}

func tryActivateProfile(endpoint string) bool {
	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader("{}"))
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:147.0) Gecko/20100101 Firefox/147.0")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	resp, err := DoRequest(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

func tryListMultiprofile(accountID string) ([]CRProfile, bool) {
	body, err := accountsGET(multiprofileURL(accountID))
	if err != nil {
		return nil, false
	}
	profiles := parseCRProfiles(body)
	if len(profiles) == 0 {
		return nil, false
	}
	return profiles, true
}

func tryListFromMe() ([]CRProfile, bool) {
	body, err := accountsGET(accountsMeURL)
	if err != nil {
		return nil, false
	}
	// /me may embed multiprofile items or a single selected profile.
	profiles := parseCRProfiles(body)
	if len(profiles) > 0 {
		return profiles, true
	}
	// Single-profile fallback from common /me fields.
	if p, ok := parseMeAsSingleProfile(body); ok {
		return []CRProfile{p}, true
	}
	return nil, false
}

func accountsGET(endpoint string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:147.0) Gecko/20100101 Firefox/147.0")
	req.Header.Set("Accept", "application/json")
	resp, err := DoRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("accounts GET HTTP %d", resp.StatusCode)
	}
	return body, nil
}

// parseCRProfiles extracts multiprofile entries from flexible JSON shapes.
func parseCRProfiles(body []byte) []CRProfile {
	// Shape A: { "items": [ ... ] } or { "data": [ ... ] } or bare array.
	var wrapper struct {
		Items []json.RawMessage `json:"items"`
		Data  []json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &wrapper); err == nil {
		raws := wrapper.Items
		if len(raws) == 0 {
			raws = wrapper.Data
		}
		if len(raws) > 0 {
			return decodeProfileList(raws)
		}
	}

	// Nested: multiprofiles / profiles key.
	var nested map[string]json.RawMessage
	if err := json.Unmarshal(body, &nested); err == nil {
		for _, key := range []string{"multiprofiles", "multiprofile", "profiles", "profile_list"} {
			if raw, ok := nested[key]; ok {
				var arr []json.RawMessage
				if json.Unmarshal(raw, &arr) == nil && len(arr) > 0 {
					return decodeProfileList(arr)
				}
			}
		}
	}

	// Bare array.
	var bare []json.RawMessage
	if err := json.Unmarshal(body, &bare); err == nil && len(bare) > 0 {
		return decodeProfileList(bare)
	}
	return nil
}

func decodeProfileList(raws []json.RawMessage) []CRProfile {
	out := make([]CRProfile, 0, len(raws))
	for _, raw := range raws {
		if p, ok := decodeOneCRProfile(raw); ok {
			out = append(out, p)
		}
	}
	return out
}

func decodeOneCRProfile(raw json.RawMessage) (CRProfile, bool) {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return CRProfile{}, false
	}
	id := firstString(m, "profile_id", "profileId", "id", "external_id", "account_id")
	name := firstString(m, "profile_name", "profileName", "name", "username", "maturity_name")
	if id == "" {
		return CRProfile{}, false
	}
	if name == "" {
		name = id
	}
	selected := firstBool(m, "is_selected", "isSelected", "selected", "is_primary", "isPrimary")
	return CRProfile{ID: id, Name: name, IsSelected: selected}, true
}

func parseMeAsSingleProfile(body []byte) (CRProfile, bool) {
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return CRProfile{}, false
	}
	id := firstString(m, "external_id", "account_id", "profile_id", "id")
	name := firstString(m, "profile_name", "username", "email", "name")
	if id == "" {
		return CRProfile{}, false
	}
	if name == "" {
		name = id
	}
	return CRProfile{ID: id, Name: name, IsSelected: true}, true
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch t := v.(type) {
			case string:
				if s := strings.TrimSpace(t); s != "" {
					return s
				}
			}
		}
	}
	return ""
}

func firstBool(m map[string]any, keys ...string) bool {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch t := v.(type) {
			case bool:
				return t
			case string:
				switch strings.ToLower(strings.TrimSpace(t)) {
				case "true", "1", "yes":
					return true
				}
			case float64:
				return t != 0
			}
		}
	}
	return false
}
