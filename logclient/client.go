package logclient

import (
	"fmt"
	"io"
	"net/http"

	json "github.com/goccy/go-json"
)

type CtApiError struct {
	Reason         string
	HttpStatusCode int
}

func (e *CtApiError) Error() string {
	return e.Reason
}

var Logs = make(LogMap)
var httpClients []http.Client

func init() {
	httpClients = make([]http.Client, 0)
}

func AddHttpClient(client http.Client) {
	httpClients = append(httpClients, client)
}

var counter int = 0

func getRequest[T any](url string) (*T, error) {
	if len(httpClients) == 0 {
		httpClients = append(httpClients, http.Client{})
	}

	counter += 1

	var httpClientId int = counter % len(httpClients)
	httpClient := httpClients[httpClientId]
	var httpRequest *http.Request
	var err error
	if httpRequest, err = http.NewRequest(http.MethodGet, url, nil); err != nil {
		return nil, err
	}

	var httpResponse *http.Response
	if httpResponse, err = httpClient.Do(httpRequest); err != nil {
		return nil, err
	}

	defer httpResponse.Body.Close()
	var body []byte
	if body, err = io.ReadAll(httpResponse.Body); err != nil {
		return nil, err
	}

	if httpResponse.StatusCode != http.StatusOK {
		switch httpResponse.StatusCode {
		case http.StatusTooManyRequests:
			return nil, &CtApiError{Reason: fmt.Sprintf("Too Many Requests: %s", url), HttpStatusCode: httpResponse.StatusCode}
		}

		return nil, &CtApiError{Reason: fmt.Sprintf("Http Status {%d}Not OK: %s", httpResponse.StatusCode, url), HttpStatusCode: httpResponse.StatusCode}
	}

	var response T
	if err = json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response, nil
}
