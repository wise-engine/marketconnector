// Package client provides the minimal HTTP client used to talk to the Angel
// One SmartAPI. It is internal to the angelone broker implementation.
package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

// Client is a minimal HTTP client for the Angel One SmartAPI. It attaches the
// headers required by the API (API key, client IP, MAC address) to every
// request, and injects the access token once a session has been established.
type Client struct {
	httpClient  *http.Client
	accessToken string
	apiKey      string
}

// New returns a client authenticated with the given API key.
func New(apiKey string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		apiKey:     apiKey,
	}
}

// SetAccessToken sets the JWT access token attached to subsequent requests.
func (c *Client) SetAccessToken(token string) {
	c.accessToken = token
}

// SetAPIKey sets the API key used for authentication.
func (c *Client) SetAPIKey(apiKey string) {
	c.apiKey = apiKey
}

// Get issues an HTTP GET request, JSON-encodes body (if non-nil), and decodes
// the response into result.
func (c *Client) Get(url string, body any, result any) error {
	return c.do(http.MethodGet, url, body, result)
}

// Post issues an HTTP POST request, JSON-encodes body, and decodes the response
// into result.
func (c *Client) Post(url string, body any, result any) error {
	return c.do(http.MethodPost, url, body, result)
}

func (c *Client) do(method, url string, body any, result any) error {
	var payload *bytes.Buffer
	if body == nil {
		payload = bytes.NewBufferString("{}")
	} else {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		payload = bytes.NewBuffer(data)
	}

	req, err := http.NewRequest(method, url, payload)
	if err != nil {
		return err
	}
	c.addHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(bodyBytes))
	}

	if len(bodyBytes) > 0 && bodyBytes[0] == '<' {
		return fmt.Errorf("server returned HTML instead of JSON (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	return json.Unmarshal(bodyBytes, result)
}

func (c *Client) addHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-UserType", "USER")
	req.Header.Set("X-SourceID", "WEB")

	clientIP := getLocalIP()
	req.Header.Set("X-ClientLocalIP", clientIP)
	req.Header.Set("X-ClientPublicIP", clientIP)
	req.Header.Set("X-MACaddress", getMACAddress())
	req.Header.Set("X-PrivateKey", c.apiKey)

	if c.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.accessToken)
	}
}

// getLocalIP returns the machine's local IP address, or "" if it can't be
// determined.
func getLocalIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer func() { _ = conn.Close() }()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}

// getMACAddress returns the machine's MAC address, or "" if it can't be
// determined.
func getMACAddress() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp != 0 && len(iface.HardwareAddr) > 0 {
			return iface.HardwareAddr.String()
		}
	}
	return ""
}
