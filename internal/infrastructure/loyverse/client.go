package loyverse

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// newHTTPClient crea un http.Client con connection pooling para reutilizar
// conexiones TCP/TLS entre requests, evitando handshakes repetidos.
func newHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}
}

const defaultBaseURL = "https://api.loyverse.com/v1.0"

// HTTPClient es la interfaz para el cliente HTTP (permite mocking en tests).
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client es el cliente HTTP para la API de Loyverse.
type Client struct {
	httpClient HTTPClient
	baseURL    string
	token      string
}

// NewClient crea un nuevo cliente Loyverse. Retorna error si el token está vacío.
func NewClient(token string) (*Client, error) {
	if token == "" {
		return nil, fmt.Errorf("LOYVERSE_TOKEN env var not set")
	}
	return &Client{
		httpClient: newHTTPClient(),
		baseURL:    defaultBaseURL,
		token:      token,
	}, nil
}

// doRequest ejecuta una request HTTP con autenticación Bearer.
func (c *Client) doRequest(method, path string, body io.Reader) (*http.Response, error) {
	url := c.baseURL + path
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, &LoyverseAPIError{
			StatusCode: resp.StatusCode,
			Message:    string(bodyBytes),
		}
	}

	return resp, nil
}

// getJSON ejecuta un GET y decodifica la respuesta JSON en v.
func (c *Client) getJSON(path string, v interface{}) error {
	resp, err := c.doRequest(http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return json.NewDecoder(resp.Body).Decode(v)
}

// postJSON ejecuta un POST con un body JSON y decodifica la respuesta en v.
func (c *Client) postJSON(path string, body interface{}, v interface{}) error {
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshaling body: %w", err)
	}

	resp, err := c.doRequest(http.MethodPost, path, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if v != nil {
		return json.NewDecoder(resp.Body).Decode(v)
	}
	return nil
}
