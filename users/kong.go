package users

import (
	"fmt"
	"net/http"
	"net/url"
)

type kongClient interface {
	createConsumer(username, id string) error
}

type kongClientImpl struct {
	address string
}

func newKongClient(address string) kongClient {
	return kongClientImpl{address}
}

func (kong kongClientImpl) createConsumer(username, id string) error {
	var data = url.Values{
		"username":  []string{username},
		"custom_id": []string{id},
	}

	r, err := http.PostForm(fmt.Sprintf("%s/consumers", kong.address), data)
	if err != nil {
		return fmt.Errorf("could not create Kong consumer: %v", err)
	}

	if r.StatusCode != http.StatusCreated {
		return fmt.Errorf("unexpected status creating Kong consumer: %s", r.Status)
	}

	return nil
}
