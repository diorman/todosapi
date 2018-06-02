package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/diorman/todospoc"
	"github.com/diorman/todospoc/utils"
	_ "github.com/lib/pq"
)

var mainDatabaseSetup = `
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS users(
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	username TEXT NOT NULL UNIQUE,
	api_consumer_id UUID UNIQUE
);

CREATE TABLE IF NOT EXISTS lists(
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	name TEXT NOT NULL,
	owner UUID NOT NULL REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS permissions(
	user_id UUID NOT NULL REFERENCES users(id),
	list_id UUID NOT NULL REFERENCES lists(id),
	type SMALLINT,
	PRIMARY KEY (user_id, list_id)
);
`

// CREATE TABLE IF NOT EXISTS todos(
// 	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
// 	owner_id UUID NOT NULL REFERENCES users(id),
// 	name TEXT NOT NULL
// );

// CREATE TABLE IF NOT EXISTS pages_users(
// 	page_id UUID NOT NULL REFERENCES pages(id),
// 	user_id UUID NOT NULL REFERENCES users(id),
// 	type TEXT
// );

// var leadsDDL = `
// CREATE EXTENSION IF NOT EXISTS pgcrypto;

// CREATE TABLE IF NOT EXISTS leads(
// 	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
// 	page_id UUID,
// 	payload TEXT
// );
// `

func setupMainDatabase(dataSource, ddl string) error {
	db, err := utils.CreateSQLDatabaseConnection(dataSource)
	if err != nil {
		return err
	}
	defer db.Close()
	if _, err := db.Exec(ddl); err != nil {
		return fmt.Errorf("failed to execute DDLs: %v", err)
	}
	log.Println("main-database: ready")
	return nil
}

func setupKong(address string) error {
	type route struct {
		ID        string   `json:"id,omitempty"`
		Methods   []string `json:"methods"`
		Paths     []string `json:"paths"`
		StripPath bool     `json:"strip_path"`
		Service   struct {
			ID string `json:"id"`
		} `json:"service"`
	}

	type service struct {
		Name   string `json:"name"`
		Host   string `json:"host"`
		Port   int    `json:"port"`
		routes []route
	}

	services := []service{
		service{
			Name: "users",
			Host: "users",
			Port: 8080,
			routes: []route{
				route{
					Methods:   []string{"POST"},
					Paths:     []string{"/users"},
					StripPath: false,
				},
				route{
					Methods:   []string{"POST"},
					Paths:     []string{"/login"},
					StripPath: false,
				},
			},
		},
	}

	for _, svc := range services {
		{
			// delete service routes
			res, err := http.Get(fmt.Sprintf("%s/services/%s/routes", address, svc.Name))
			if err != nil {
				return fmt.Errorf("failed to fetch routes for service %s: %v", svc.Name, err)
			}
			defer res.Body.Close()

			resBody := struct {
				Data []route `json:"data"`
			}{}
			decoder := json.NewDecoder(res.Body)
			decoder.Decode(&resBody)

			for _, r := range resBody.Data {
				req, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/routes/%s", address, r.ID), nil)
				res, err := http.DefaultClient.Do(req)
				if err != nil {
					return fmt.Errorf("failed to delete route for service %s: %v", svc.Name, err)
				}
				if res.StatusCode != http.StatusNoContent {
					return fmt.Errorf("unexpected status code deleteing route for service service %s: %v", svc.Name, res.StatusCode)
				}
				log.Printf("kong: route deleted %v %v (%s)\n", r.Methods, r.Paths, svc.Name)
			}
		}

		{
			// delete service
			req, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/services/%s", address, svc.Name), nil)
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				return fmt.Errorf("failed to delete service %s: %v", svc.Name, err)
			}
			if res.StatusCode == http.StatusNoContent {
				log.Printf("kong: service deleted '%v'", svc.Name)
			}
		}

		{
			// create service
			reqBody, err := json.Marshal(svc)
			if err != nil {
				return fmt.Errorf("failed to encode service body %s: %v", svc.Host, err)
			}
			req, err := http.Post(fmt.Sprintf("%s/services", address), "application/json", bytes.NewBuffer(reqBody))
			if err != nil {
				return fmt.Errorf("failed to add service %s: %v", svc.Host, err)
			}
			defer req.Body.Close()
			if req.StatusCode != http.StatusCreated {
				return fmt.Errorf("unexpected status code creating service %s: %v", svc.Host, req.StatusCode)
			}
			log.Printf("kong: service created '%v'", svc.Name)
			decoder := json.NewDecoder(req.Body)
			resBody := struct {
				ID string `json:"id"`
			}{}
			if err := decoder.Decode(&resBody); err != nil {
				return fmt.Errorf("failed to decode response when creating service %s: %v", svc.Host, err)
			}
			// create routes
			for _, r := range svc.routes {
				r.Service.ID = resBody.ID
				reqBody, err := json.Marshal(r)
				if err != nil {
					return fmt.Errorf("failed to encode route body for service %s: %v", svc.Host, err)
				}
				req, err := http.Post(fmt.Sprintf("%s/routes", address), "application/json", bytes.NewBuffer(reqBody))
				if err != nil {
					return fmt.Errorf("failed to add service %s: %v", svc.Host, err)
				}
				defer req.Body.Close()
				if req.StatusCode != http.StatusCreated {
					return fmt.Errorf("unexpected status code creating route for service %s: %v", svc.Host, req.StatusCode)
				}
				log.Printf("kong: route created %v %v (%s)\n", r.Methods, r.Paths, svc.Name)
			}
		}
	}

	return nil
}

func setupSQS() error {
	client, err := utils.NewSQSClient()
	if err != nil {
		return err
	}
	queueNames := []string{
		todospoc.Config.UserEventsQueueName,
	}
	for _, queueName := range queueNames {
		for i := 0; i < 3; i++ {
			res, err := client.CreateQueue(queueName)
			if err != nil {
				log.Printf("failed to create queue %s: %v\n", queueName, err)
				time.Sleep(1 * time.Second)
				continue
			}
			log.Printf("sqs: queue created %s", *res.QueueUrl)
			break
		}
	}
	return nil
}

func main() {
	if err := setupMainDatabase(todospoc.Config.MainSQLDBSource, mainDatabaseSetup); err != nil {
		log.Fatalf("main DB error: %v\n", err)
	}

	if err := setupKong(todospoc.Config.KongAdminAddress); err != nil {
		log.Fatalf("kong error: %v\n", err)
	}

	if err := setupSQS(); err != nil {
		log.Fatalf("kong error: %v\n", err)
	}

	log.Println("setup complete")
}
