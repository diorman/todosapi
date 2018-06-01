package users

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/diorman/todospoc/utils"
	"github.com/julienschmidt/httprouter"
	"github.com/lib/pq"
)

type handler struct {
	*httprouter.Router
	db    *sql.DB
	store store
	kong  kongClient
}

func NewHandler(r *httprouter.Router, db *sql.DB, kongAddress string) handler {
	var (
		k = newKongClient(kongAddress)
		s = newStore()
	)
	return newHandler(r, db, s, k)
}

func newHandler(r *httprouter.Router, db *sql.DB, s store, k kongClient) handler {
	h := handler{
		Router: r,
		db:     db,
		store:  s,
		kong:   k,
	}
	h.setupRoutes()
	return h
}

func (h handler) handleCreateUser() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		var (
			userID      string
			username    string
			decoder     = json.NewDecoder(r.Body)
			requestBody = struct {
				Username string `json:"username"`
			}{}
		)

		if err := decoder.Decode(&requestBody); err != nil {
			log.Println(err)
			utils.WriteJSON(w,
				http.StatusInternalServerError,
				errors.New(http.StatusText(http.StatusInternalServerError)))
			return
		}

		if username = strings.TrimSpace(requestBody.Username); username == "" {
			utils.WriteJSON(w,
				http.StatusBadRequest,
				errors.New("username can't be empty"))
			return
		}

		err := utils.SQLTransact(h.db, func(tx *sql.Tx) error {
			id, err := h.store.saveUser(tx, requestBody.Username)
			if pgerr, ok := err.(*pq.Error); ok && pgerr.Code == "23505" {
				utils.WriteJSON(w,
					http.StatusConflict,
					errors.New("username already exists"))
				return err
			}

			if err == nil {
				err = h.kong.createConsumer(requestBody.Username, id)
			}

			if err != nil {
				utils.WriteJSON(w,
					http.StatusInternalServerError,
					errors.New(http.StatusText(http.StatusInternalServerError)))
			}

			userID = id
			return err
		})

		if err != nil {
			log.Println(err)
			return
		}

		response := struct {
			ID       string `json:"id"`
			Username string `json:"username"`
		}{userID, requestBody.Username}

		utils.WriteJSON(w, http.StatusCreated, response)
	}
}

func (h handler) handleLogin() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		fmt.Fprintf(w, "hello world")
	}
}

func (h handler) handleHealthCheck() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		utils.WriteJSON(w, http.StatusOK, "OK")
	}
}

func (h handler) setupRoutes() {
	h.POST("/users", h.handleCreateUser())
	h.POST("/login", h.handleLogin())
	h.GET("/_hc", h.handleHealthCheck())
}
