package users

import (
	"database/sql"
)

type Store interface {
	saveUser(username string, txFunc func(userID string) error) (string, error)
	getConsumerID(username string) (string, error)
	setConsumerID(userID, consumerID string) error
}

type storeImpl struct {
	*sql.DB
}

func NewStore(db *sql.DB) Store {
	return &storeImpl{db}
}

func (s *storeImpl) saveUser(username string, txFunc func(userID string) error) (string, error) {
	tx, err := s.Begin()
	if err != nil {
		return "", err
	}

	handleError := func(tx *sql.Tx, err error) error {
		if rbErr := tx.Rollback(); rbErr != nil {
			return rbErr
		}
		return err
	}

	stmt, err := tx.Prepare("INSERT INTO users(username) VALUES($1) RETURNING id")
	if err != nil {
		return "", handleError(tx, err)
	}

	var userID string
	if err := stmt.QueryRow(username).Scan(&userID); err != nil {
		return "", handleError(tx, err)
	}

	if err := txFunc(userID); err != nil {
		return "", handleError(tx, err)
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}

	return userID, nil
}

func (s *storeImpl) getConsumerID(username string) (string, error) {
	var consumerID string
	row := s.QueryRow("SELECT api_consumer_id FROM users WHERE username=$1", username)
	if err := row.Scan(&consumerID); err != nil {
		return "", err
	}
	return consumerID, nil
}

func (s *storeImpl) setConsumerID(userID, consumerID string) error {
	if _, err := s.Exec("UPDATE users SET api_consumer_id=$1 WHERE id=$2", consumerID, userID); err != nil {
		return err
	}
	return nil
}
