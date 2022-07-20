package mpesa

import (
	"bytes"
	"io"
	"net/http"
)

type mockResponseFunc func() (status int, body string)

type (
	mockResponse struct {
		fn mockResponseFunc
	}

	mockHttpClient struct {
		requests map[string]mockResponse
	}
)

// mockHttpResponse returns a http.Response with the given status and body.
func mockHttpResponse(status int, body string) *http.Response {
	return &http.Response{
		Status:     http.StatusText(status),
		StatusCode: status,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Body:       io.NopCloser(bytes.NewBuffer([]byte(body))),
	}
}

// newMockHttpClient creates a new instance of mockHttpClient
func newMockHttpClient() *mockHttpClient {
	return &mockHttpClient{
		requests: make(map[string]mockResponse),
	}
}

// MockRequest appends the given response for the provided url.
func (m *mockHttpClient) MockRequest(url string, fn mockResponseFunc) {
	m.requests[url] = mockResponse{fn: fn}
}

// Do checks if the given req.URL exists in the available requests lists and returns the stored response.
// If none exists, it returns status http.StatusNotFound
func (m *mockHttpClient) Do(req *http.Request) (*http.Response, error) {
	if mock, ok := m.requests[req.URL.String()]; ok {
		if mock.fn != nil {
			status, body := mock.fn()
			return mockHttpResponse(status, body), nil
		}
	}

	return mockHttpResponse(http.StatusNotFound, http.StatusText(http.StatusNotFound)), nil
}
