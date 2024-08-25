package outlineApi

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
)

type AccessKey struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Password  string `json:"password"`
	Port      int    `json:"port"`
	Method    string `json:"method"`
	AccessUrl string `json:"accessUrl"`
	DataLimit struct {
		Bytes int64 `json:"bytes"`
	} `json:"dataLimit,omitempty"`
}


func OutlineApiCall(method string, url string, result any)(any, error){
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func ListAccessKeys() ([]AccessKey, error) {
	var result struct {
		AccessKeys []AccessKey `json:"accessKeys"`
	}
	resp, err := OutlineApiCall("GET","https://backup.heycodinguy.site:3843/tVhaFf05N6k8tHKXk3UZ-w/access-keys/", &result)
	if err != nil {
		fmt.Println("Error:", err)
		return nil, err
	}
	accessKeysResult, ok := resp.(*struct {
		AccessKeys []AccessKey `json:"accessKeys"`
	})
	if !ok {
		fmt.Println("Error: Type assertion failed")
		return nil, err
	}

	return accessKeysResult.AccessKeys, nil
}

func CreateAccessKey(apiURL string) (*AccessKey, error) {
    req, err := http.NewRequest("POST", apiURL+"/access-keys", nil)
    if err != nil {
        return nil, err
    }

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var key AccessKey
    err = json.NewDecoder(resp.Body).Decode(&key)
    if err != nil {
        return nil, err
    }

    return &key, nil
}

// GetAccessKey retrieves details of a specific access key by its ID.
func GetAccessKey(apiURL, keyID string) (*AccessKey, error) {
    resp, err := http.Get(apiURL + "/access-keys/" + keyID)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var key AccessKey
    err = json.NewDecoder(resp.Body).Decode(&key)
    if err != nil {
        return nil, err
    }

    return &key, nil
}

// RenameAccessKey renames an existing access key on the Outline server.
func RenameAccessKey(apiURL, keyID, newName string) error {
    req, err := http.NewRequest("PUT", apiURL+"/access-keys/"+keyID+"/name", strings.NewReader("name="+newName))
    if err != nil {
        return err
    }

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        bodyBytes, _ := ioutil.ReadAll(resp.Body)
        return fmt.Errorf("failed to rename access key: %s", string(bodyBytes))
    }

    return nil
}

// DeleteAccessKey deletes an access key from the Outline server.
func DeleteAccessKey(apiURL, keyID string) error {
    req, err := http.NewRequest("DELETE", apiURL+"/access-keys/"+keyID, nil)
    if err != nil {
        return err
    }

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        bodyBytes, _ := ioutil.ReadAll(resp.Body)
        return fmt.Errorf("failed to delete access key: %s", string(bodyBytes))
    }

    return nil
}

// SetDataLimit sets a data transfer limit for all access keys on the Outline server.
func SetDataLimit(apiURL string, limit int64) error {
    jsonData := fmt.Sprintf(`{"limit": {"bytes": %d}}`, limit)
    req, err := http.NewRequest("PUT", apiURL+"/server/access-key-data-limit", strings.NewReader(jsonData))
    if err != nil {
        return err
    }
    req.Header.Set("Content-Type", "application/json")

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        bodyBytes, _ := ioutil.ReadAll(resp.Body)
        return fmt.Errorf("failed to set data limit: %s", string(bodyBytes))
    }

    return nil
}

// RemoveDataLimit removes the data transfer limit for all access keys on the Outline server.
func RemoveDataLimit(apiURL string) error {
    req, err := http.NewRequest("DELETE", apiURL+"/server/access-key-data-limit", nil)
    if err != nil {
        return err
    }

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        bodyBytes, _ := ioutil.ReadAll(resp.Body)
        return fmt.Errorf("failed to remove data limit: %s", string(bodyBytes))
    }

    return nil
}
