package outlineApi

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"
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

// this type uses in outline config -> v2ray subscription config convertor.
type V2RayConfig struct {
	V          string `json:"v"`
	Ps         string `json:"ps"`
	Add        string `json:"add"`
	Port       string `json:"port"`
	Method     string `json:"method"`
	Password   string `json:"password"`
	Plugin     string `json:"plugin,omitempty"`
	PluginOpts string `json:"plugin_opts,omitempty"`
}

type AccessKeysUsage struct {
	BytesTransferredByUserID map[string]int64 `json:"bytesTransferredByUserId"`
}

func OutlineApiCall(method string, url string, requestBody interface{}, result any) (any, error) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	var bodyReader *bytes.Reader
	if requestBody != nil {
		// Marshal the requestBody into JSON if it's not nil
		jsonBody, err := json.Marshal(requestBody)
		if err != nil {
			return nil, fmt.Errorf("error marshaling request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)

		// Print the JSON body for debugging
		fmt.Println("Request body JSON:", string(jsonBody))
	} else {
		bodyReader = nil
	}

	// Create the HTTP request
	var req *http.Request
	var err error
	if bodyReader != nil {
		req, err = http.NewRequest(method, url, bodyReader)

	} else {
		req, err = http.NewRequest(method, url, nil)
	}

	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	// Set the Content-Type header to application/json
	if requestBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	// Print the response body for debugging
	fmt.Println("Response body JSON:", string(respBody))

	// Unmarshal the response body if a result is expected
	if len(string(respBody)) != 0 {
		err = json.Unmarshal(respBody, &result)
		if err != nil {
			return nil, fmt.Errorf("error unmarshaling response body: %w", err)
		}
	}

	return result, nil
}

func CheckServerHealth(apiURL string) (bool, error) {
	_, err := ListAccessKeys(apiURL)
	if err != nil {
		return false, err
	}

	return true, nil
}

func ListAccessKeys(apiURL string) ([]AccessKey, error) {
	var result struct {
		AccessKeys []AccessKey `json:"accessKeys"`
	}
	_, err := OutlineApiCall("GET", apiURL+"/access-keys/", nil, &result)
	if err != nil {
		fmt.Println("Error:", err)
		return nil, err
	}
	return result.AccessKeys, nil
}

func CreateAccessKey(apiURL string, name string, limit int64) (*AccessKey, error) {
	type CreateData struct {
		Name  string `json:"name"`
		Limit struct {
			Bytes int64 `json:"bytes"`
		} `json:"limit"`
	}
	var result AccessKey
	createData := CreateData{
		Name: name,
		Limit: struct {
			Bytes int64 `json:"bytes"`
		}{
			Bytes: limit * 1e+9,
		},
	}

	// Perform the API call and unmarshal the response into result
	_, err := OutlineApiCall("POST", apiURL+"/access-keys/", createData, &result)
	if err != nil {
		return nil, err
	}

	// Return the address of the result variable
	return &result, nil
}

// GetAccessKey retrieves details of a specific access key by its ID.
func GetAccessKey(apiURL, keyID string) (*AccessKey, error) {
	var result AccessKey
	_, err := OutlineApiCall("GET", apiURL+"/access-keys/"+keyID, nil, &result)
	if err != nil {
		return nil, err
	}

	return &result, nil
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
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to rename access key: %s", string(bodyBytes))
	}

	return nil
}

func DeleteAccessKey(apiURL, keyID string) error {
	var res any
	res, err := OutlineApiCall("DELETE", apiURL+"/access-keys/"+string(keyID), nil, &res)
	if err != nil {
		fmt.Println("Error deleting access key:", err)
		return err
	}
	fmt.Println("Delete response:", &res)
	return nil
}

func GetAccessKeysUsages(apiURL string) (*AccessKeysUsage, error) {
	var result AccessKeysUsage
	_, err := OutlineApiCall("GET", apiURL+"/metrics/transfer/", nil, &result)
	if err != nil {
		return nil, err
	}

	return &result, nil
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

func ParseOutlineSS(accessURL string) (V2RayConfig, error) {
	parts := strings.Split(accessURL, "ss://")
	if len(parts) < 2 {
		return V2RayConfig{}, echo.NewHTTPError(http.StatusBadRequest, "Invalid Outline SS URL")
	}

	// Remove query params
	ssMain := strings.Split(parts[1], "/?")[0]
	hash := strings.Split(ssMain, "@")
	// Base64 decode the SS details
	decoded, err := base64.StdEncoding.DecodeString(hash[0])
	if err != nil {
		return V2RayConfig{}, echo.NewHTTPError(http.StatusBadRequest, "Failed to decode SS URL")
	}

	// Split extracted data (format: method:password@host:port)
	credentials := strings.Split(string(decoded), "@")
	if len(credentials) < 1 {
		return V2RayConfig{}, echo.NewHTTPError(http.StatusBadRequest, "Invalid SS format")
	}

	auth := strings.Split(credentials[0], ":")
	hostPort := strings.Split(hash[1], ":")
	fmt.Printf("decoded: %s,hash: %s\n\nauth: %s, hostport: %s", decoded, hash, auth, hostPort)

	// if len(auth) < 2 || len(hostPort) < 2 {
	// 	return V2RayConfig{}, fmt.Errorf("invalid SS structure")
	// }

	// Build V2Ray config
	return V2RayConfig{
		V:          "2",
		Ps:         "Outline-V2Ray",
		Add:        hostPort[0],
		Port:       hostPort[1],
		Method:     auth[0],
		Password:   auth[1],
		Plugin:     "v2ray-plugin",
		PluginOpts: "mode=websocket;tls",
	}, nil
}
