package client

import (
	"net/http"

	"github.com/stretchr/testify/mock"
)

// MockHTTP is a mock http client
type MockHTTP struct {
	mock.Mock
}

// Do implements the HTTP interface method
func (m *MockHTTP) Do(r, pr *http.Request) (int, []byte, map[string]string, map[string]string, error) {
	args := m.Called(r, pr)

	if d := args.Get(1); d != nil {
		return args.Int(0), d.([]byte), nil, nil, args.Error(2)
	}

	return args.Int(0), nil, nil, nil, args.Error(2)
}
