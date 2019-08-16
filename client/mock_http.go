package client

import "github.com/stretchr/testify/mock"

// MockHTTP is a mock http client
type MockHTTP struct {
	mock.Mock
}

// Do implements the HTTP interface method
func (m *MockHTTP) Do(uri string) ([]byte, error) {
	args := m.Called(uri)

	if d := args.Get(0); d != nil {
		return d.([]byte), nil
	}

	return nil, args.Error(1)
}
