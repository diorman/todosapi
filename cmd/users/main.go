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
	router := httprouter.New()
	h := users.NewHandler(router, db, todospoc.Config.KongAdminAddress)
	log.Println("starting users service")
	http.ListenAndServe(":8080", h)
}
