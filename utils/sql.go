package utils

import (
	"database/sql"
	"fmt"
)

func CreateSQLDatabaseConnection(dataSource string) (*sql.DB, error) {
	db, err := sql.Open("postgres", dataSource)
	if err != nil {
		return nil, fmt.Errorf("could not open open database: %v", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("could not establish connection with db: %v", err)
	}

	return db, nil
}

func SQLTransact(db *sql.DB, txFunc func(*sql.Tx) error) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	err = txFunc(tx)
	if err != nil {
		if e := tx.Rollback(); e != nil {
			return e
		}
		return err
	}
	return tx.Commit()
}
