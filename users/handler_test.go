package users

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/julienschmidt/httprouter"
	"github.com/lib/pq"
)

type testStore struct {
	saveUserReturn struct {
		userID string
		err    error
	}
	getConsumerIDReturn struct {
		consumerID string
		err        error
	}
	setConsumerIDReturn struct {
		err error
	}
}

func (s *testStore) saveUser(username string, txFunc func(userID string) error) (string, error) {
	return s.saveUserReturn.userID, s.saveUserReturn.err
}

func (s *testStore) getConsumerID(username string) (string, error) {
	return s.getConsumerIDReturn.consumerID, s.getConsumerIDReturn.err
}
func (s *testStore) setConsumerID(userID, consumerID string) error {
	return s.setConsumerIDReturn.err
}

type testKongClient struct {
	getJWTCredentialsReturn struct {
		jwtCredentials []JWTCredentials
		err            error
	}
}

func (kong *testKongClient) createConsumer(userID string) (string, error) {
	return "", nil
}

func (kong *testKongClient) createJWTCredentials(consumerID string) error {
	return nil
}

func (kong *testKongClient) getJWTCredentials(customID string) ([]JWTCredentials, error) {
	return kong.getJWTCredentialsReturn.jwtCredentials, kong.getJWTCredentialsReturn.err
}

var testJWTCredentials = JWTCredentials{
	Key:       "c5a55906cc244f483226e02bcff2b5e",
	Algorithm: "HS256",
	Secret:    "b0970f7fc9564e65xklfn48930b5d08b1",
}

var testJWT = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJjNWE1NTkwNmNjMjQ0ZjQ4MzIyNmUwMmJjZmYyYjVlIn0.DcsoWhy98uc3GVyLCg-sytctfgLHEw6rWNjnWiZa8nA"

type testSQSClient struct {
}

func (c *testSQSClient) SendMessage(queueName string, messageBody interface{}) error {
	return nil
}

func (c *testSQSClient) CreateQueue(queueName string) (*sqs.CreateQueueOutput, error) {
	return nil, nil
}

func (c *testSQSClient) ReceiveMessage(queueName string, maxMessages, waitTime int64) ([]sqs.Message, error) {
	return nil, nil
}

func (c *testSQSClient) DeleteMessage(queueName string, receiptHandle string) error {
	return nil
}

func TestHandleCreateUser(t *testing.T) {
	tests := map[string]struct {
		requestBody               string
		statusCode                int
		responseBody              string
		storeSaveUserReturnUserID string
		storeSaveUserReturnErr    error
	}{
		"returns 201 and successful response when created": {
			requestBody:               `{"username": "user-123"}`,
			statusCode:                http.StatusCreated,
			responseBody:              `{"id":"123","username":"user-123"}`,
			storeSaveUserReturnUserID: "123",
		},
		"returns 500 and error response when store fails": {
			requestBody:            `{"username": "user-123"}`,
			statusCode:             http.StatusInternalServerError,
			responseBody:           fmt.Sprintf(`{"error":"%s"}`, http.StatusText(http.StatusInternalServerError)),
			storeSaveUserReturnErr: errors.New("server error"),
		},
		"returns 400 and error response when username is not present in request": {
			requestBody:  `{}`,
			statusCode:   http.StatusBadRequest,
			responseBody: `{"error":"username can't be empty"}`,
		},
		"returns 409 and error response when username already exists": {
			requestBody:            `{"username": "user-123"}`,
			statusCode:             http.StatusConflict,
			responseBody:           `{"error":"username already exists"}`,
			storeSaveUserReturnErr: &pq.Error{Code: "23505"},
		},
	}

	for td, tt := range tests {
		s := testStore{}
		s.saveUserReturn.userID = tt.storeSaveUserReturnUserID
		s.saveUserReturn.err = tt.storeSaveUserReturnErr

		h := NewHandler(httprouter.New(), &s, &testKongClient{}, &testSQSClient{})
		r, _ := http.NewRequest("POST", "/users", bytes.NewBuffer([]byte(tt.requestBody)))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)

		if tt.statusCode != w.Code {
			t.Errorf("%v: handler returned wrong status code: expected %v, got %v", td, tt.statusCode, w.Code)
		}

		if tt.responseBody != w.Body.String() {
			t.Errorf("%v: handler returned wrong body: expected %v, got %v", td, tt.responseBody, w.Body.String())
		}
	}
}

func TestHandleLogin(t *testing.T) {
	tests := map[string]struct {
		requestBody                      string
		statusCode                       int
		responseBody                     string
		storeGetConsumerIDConsumerID     string
		storeGetConsumerIDError          error
		kongGetJWTCredentialsReturnCreds []JWTCredentials
		kongGetJWTCredentialsReturnError error
	}{
		"returns 200 and successful response when valid": {
			requestBody:                      `{"username": "user-123"}`,
			statusCode:                       http.StatusOK,
			responseBody:                     fmt.Sprintf(`{"jwt":"%v"}`, testJWT),
			storeGetConsumerIDConsumerID:     "123",
			kongGetJWTCredentialsReturnCreds: []JWTCredentials{testJWTCredentials},
		},
		"returns 401 and error response when username does not exist": {
			requestBody:             `{"username": "user-123"}`,
			statusCode:              http.StatusUnauthorized,
			responseBody:            fmt.Sprintf(`{"error":"%s"}`, http.StatusText(http.StatusUnauthorized)),
			storeGetConsumerIDError: sql.ErrNoRows,
		},
		"returns 500 and error response when cannot fetch jwt credentials": {
			requestBody:                      `{"username": "user-123"}`,
			statusCode:                       http.StatusInternalServerError,
			responseBody:                     fmt.Sprintf(`{"error":"%s"}`, http.StatusText(http.StatusInternalServerError)),
			storeGetConsumerIDConsumerID:     "123",
			kongGetJWTCredentialsReturnError: errors.New("failed to fetch credentials"),
		},
		"returns 500 and error response when no jwt credentials were found": {
			requestBody:                      `{"username": "user-123"}`,
			statusCode:                       http.StatusInternalServerError,
			responseBody:                     fmt.Sprintf(`{"error":"%s"}`, http.StatusText(http.StatusInternalServerError)),
			storeGetConsumerIDConsumerID:     "123",
			kongGetJWTCredentialsReturnCreds: []JWTCredentials{},
		},
		"returns 400 and error response when username is not present in request": {
			requestBody:  `{}`,
			statusCode:   http.StatusBadRequest,
			responseBody: `{"error":"username can't be empty"}`,
		},
	}

	for td, tt := range tests {
		s := testStore{}
		s.getConsumerIDReturn.consumerID = tt.storeGetConsumerIDConsumerID
		s.getConsumerIDReturn.err = tt.storeGetConsumerIDError

		k := testKongClient{}
		k.getJWTCredentialsReturn.jwtCredentials = tt.kongGetJWTCredentialsReturnCreds
		k.getJWTCredentialsReturn.err = tt.kongGetJWTCredentialsReturnError

		h := NewHandler(httprouter.New(), &s, &k, &testSQSClient{})
		r, _ := http.NewRequest("POST", "/login", bytes.NewBuffer([]byte(tt.requestBody)))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)

		if tt.statusCode != w.Code {
			t.Errorf("%v: handler returned wrong status code: expected %v, got %v", td, tt.statusCode, w.Code)
		}

		if tt.responseBody != w.Body.String() {
			t.Errorf("%v: handler returned wrong body: expected %v, got %v", td, tt.responseBody, w.Body.String())
		}
	}
}

func TestCraftJWT(t *testing.T) {
	jwt, err := craftJWT(testJWTCredentials)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if jwt != testJWT {
		t.Errorf("unexpected JWT: %v", jwt)
	}
}
