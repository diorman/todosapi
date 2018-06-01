package main

import (
	"fmt"
	"log"

	"github.com/diorman/todospoc"
	"github.com/diorman/todospoc/utils"
	_ "github.com/lib/pq"
)

var mainDatabaseSetup = `
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS users(
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	username TEXT NOT NULL UNIQUE
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

	return nil
}

func main() {
	if err := setupMainDatabase(todospoc.Config.MainSQLDBSource, mainDatabaseSetup); err != nil {
		log.Fatalf("main DB error: %v\n", err)
	}

	fmt.Println("setup complete")
}
