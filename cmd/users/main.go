package main

import (
	"log"
	"net/http"

	"github.com/diorman/todospoc"
	"github.com/diorman/todospoc/users"
	"github.com/diorman/todospoc/utils"

	"github.com/julienschmidt/httprouter"

	_ "github.com/lib/pq"
)

func main() {
	db, err := utils.CreateSQLDatabaseConnection(todospoc.Config.MainSQLDBSource)
	if err != nil {
		log.Fatalf("%v\n", err)
	}
	sqs, err := utils.NewSQSClient()
	if err != nil {
		log.Fatalf("%v\n", err)
	}
	var (
		router = httprouter.New()
		store  = users.NewStore(db)
		kong   = users.NewKongClient(todospoc.Config.KongAdminAddress)
		h      = users.NewHandler(router, store, kong, sqs)
		w      = users.NewWorker(sqs, store, kong, todospoc.Config.UserEventsQueueName)
	)
	go w.Start()
	log.Println("starting users service")
	http.ListenAndServe(":8080", h)
}
