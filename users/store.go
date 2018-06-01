package users

import (
	"database/sql"
)

type store interface {
	saveUser(tx *sql.Tx, username string) (string, error)
}

type storeImpl struct{}

func newStore() store {
	return storeImpl{}
}

func (s storeImpl) saveUser(tx *sql.Tx, username string) (string, error) {
	stmt, err := tx.Prepare("INSERT INTO users(username) VALUES($1) RETURNING id")
	if err != nil {
		return "", err
	}
	defer stmt.Close()

	var userID string
	if err := stmt.QueryRow(username).Scan(&userID); err != nil {
		return "", err
	}

	return userID, nil
}
