package users

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

type JWTCredentials struct {
	Key       string `json:"key"`
	Algorithm string `json:"algorithm"`
	Secret    string `json:"secret"`
}

type KongClient interface {
	createConsumer(userID string) (string, error)
	createJWTCredentials(consumerID string) error
	getJWTCredentials(consumerID string) ([]JWTCredentials, error)
}

type kongClientImpl struct {
	address string
}

func NewKongClient(address string) KongClient {
	return &kongClientImpl{address}
}

func (kong *kongClientImpl) createConsumer(userID string) (string, error) {
	body := []byte(fmt.Sprintf(`{"custom_id": "%v"}`, userID))

	r, err := http.Post(fmt.Sprintf("%s/consumers", kong.address), "application/json", bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("could not create Kong consumer: %v", err)
	}
	defer r.Body.Close()

	responseBody, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return "", fmt.Errorf("could not read response body: %v", err)
	}

	if r.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("unexpected response creating Kong consumer: %d %s", r.StatusCode, string(responseBody))
	}

	response := struct {
		ID string `json:"id"`
	}{}

	if err := json.Unmarshal(responseBody, &response); err != nil {
		return "", fmt.Errorf("failed to decode response body: %v", err)
	}

	return response.ID, nil
}

func (kong *kongClientImpl) createJWTCredentials(consumerID string) error {
	r, err := http.Post(fmt.Sprintf("%s/consumers/%s/jwt", kong.address, consumerID), "application/json", bytes.NewBuffer([]byte("{}")))
	if err != nil {
		return fmt.Errorf("could not create Kong JWT credentials: %v", err)
	}
	defer r.Body.Close()

	responseBody, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("could not read response body: %v", err)
	}

	if r.StatusCode != http.StatusCreated {
		return fmt.Errorf("unexpected response creating Kong consumer: %d %s", r.StatusCode, string(responseBody))
	}

	return nil
}

func (kong *kongClientImpl) getJWTCredentials(consumerID string) ([]JWTCredentials, error) {
	r, err := http.Get(fmt.Sprintf("%s/consumers/%s/jwt", kong.address, consumerID))
	if err != nil {
		return nil, fmt.Errorf("could not fetch Kong JWT credentials: %v", err)
	}

	defer r.Body.Close()

	var (
		decoder      = json.NewDecoder(r.Body)
		responseBody = struct {
			Data []JWTCredentials `json:"data"`
		}{}
	)

	if err := decoder.Decode(&responseBody); err != nil {
		return nil, fmt.Errorf("could not decode kong JWT credentials response: %v", err)
	}

	return responseBody.Data, nil
}
