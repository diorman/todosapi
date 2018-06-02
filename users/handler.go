package users

import (
	"crypto"
	"crypto/hmac"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/diorman/todospoc"

	"github.com/diorman/todospoc/utils"
	"github.com/julienschmidt/httprouter"
	"github.com/lib/pq"
)

type Handler struct {
	*httprouter.Router
	store Store
	kong  KongClient
	sqs   utils.SQSClient
}

func NewHandler(r *httprouter.Router, s Store, k KongClient, sqs utils.SQSClient) Handler {
	h := Handler{
		Router: r,
		store:  s,
		kong:   k,
		sqs:    sqs,
	}
	h.setupRoutes()
	return h
}

func (h Handler) handleCreateUser() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		var (
			username    string
			decoder     = json.NewDecoder(r.Body)
			requestBody = struct {
				Username string `json:"username"`
			}{}
		)

		if err := decoder.Decode(&requestBody); err != nil {
			log.Println(err)
			utils.WriteStandardErrorJSON(w, http.StatusInternalServerError)
			return
		}

		if username = strings.TrimSpace(requestBody.Username); username == "" {
			utils.WriteJSON(w, http.StatusBadRequest, errors.New("username can't be empty"))
			return
		}

		userID, err := h.store.saveUser(username, func(userID string) error {
			messageBody := struct {
				EventType string `json:"event_type"`
				Payload   struct {
					UserID string `json:"user_id"`
				} `json:"payload"`
			}{}
			messageBody.EventType = "user_created"
			messageBody.Payload.UserID = userID
			if err := h.sqs.SendMessage(todospoc.Config.UserEventsQueueName, messageBody); err != nil {
				return err
			}
			return nil
		})

		if pgerr, ok := err.(*pq.Error); ok && pgerr.Code == "23505" {
			utils.WriteJSON(w, http.StatusConflict, errors.New("username already exists"))
			return
		}

		if err != nil {
			log.Println(err)
			utils.WriteStandardErrorJSON(w, http.StatusInternalServerError)
			return
		}

		response := struct {
			ID       string `json:"id"`
			Username string `json:"username"`
		}{userID, requestBody.Username}

		utils.WriteJSON(w, http.StatusCreated, response)
	}
}

func (h Handler) handleLogin() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		var (
			username    string
			decoder     = json.NewDecoder(r.Body)
			requestBody = struct {
				Username string `json:"username"`
			}{}
		)

		if err := decoder.Decode(&requestBody); err != nil {
			log.Println(err)
			utils.WriteStandardErrorJSON(w, http.StatusInternalServerError)
			return
		}

		if username = strings.TrimSpace(requestBody.Username); username == "" {
			utils.WriteJSON(w, http.StatusBadRequest, errors.New("username can't be empty"))
			return
		}

		consumerID, err := h.store.getConsumerID(username)

		if err == sql.ErrNoRows {
			utils.WriteStandardErrorJSON(w, http.StatusUnauthorized)
			return
		}
		if err != nil {
			log.Println(err)
			utils.WriteStandardErrorJSON(w, http.StatusInternalServerError)
			return
		}

		jwtCredentials, err := h.kong.getJWTCredentials(consumerID)
		if err != nil {
			log.Println(err)
			utils.WriteStandardErrorJSON(w, http.StatusInternalServerError)
			return
		}

		if len(jwtCredentials) == 0 {
			log.Printf("no jwt credentials found for username: %v\n", username)
			utils.WriteStandardErrorJSON(w, http.StatusInternalServerError)
			return
		}

		jwt, err := craftJWT(jwtCredentials[0])
		if err != nil {
			log.Println(err)
			utils.WriteStandardErrorJSON(w, http.StatusInternalServerError)
			return
		}

		response := struct {
			JWT string `json:"jwt"`
		}{jwt}

		utils.WriteJSON(w, http.StatusOK, response)
	}
}

func craftJWT(creds JWTCredentials) (string, error) {
	header, err := json.Marshal(struct {
		Alg string `json:"alg"`
		Typ string `json:"typ"`
	}{creds.Algorithm, "JWT"})

	if err != nil {
		return "", fmt.Errorf("could not encode JWT header: %v", err)
	}

	payload, err := json.Marshal(struct {
		ISS string `json:"iss"`
	}{creds.Key})

	if err != nil {
		return "", fmt.Errorf("could not encode JWT payload: %v", err)
	}

	encodedHeaderAndPayload := strings.Join([]string{
		strings.TrimRight(base64.URLEncoding.EncodeToString(header), "="),
		strings.TrimRight(base64.URLEncoding.EncodeToString(payload), "="),
	}, ".")

	hasher := hmac.New(crypto.SHA256.New, []byte(creds.Secret))
	hasher.Write([]byte(encodedHeaderAndPayload))
	sig := strings.TrimRight(base64.URLEncoding.EncodeToString(hasher.Sum(nil)), "=")

	return strings.Join([]string{encodedHeaderAndPayload, sig}, "."), nil
}

func (h Handler) handleHealthCheck() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		utils.WriteJSON(w, http.StatusOK, "OK")
	}
}

func (h Handler) setupRoutes() {
	h.POST("/users", h.handleCreateUser())
	h.POST("/login", h.handleLogin())
	h.GET("/_hc", h.handleHealthCheck())
}
