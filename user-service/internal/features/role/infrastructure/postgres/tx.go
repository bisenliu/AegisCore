package postgres

import "github.com/aegiscore/user-service/ent"

func rollback(tx *ent.Tx, err error) error {
	if rollbackErr := tx.Rollback(); rollbackErr != nil {
		return rollbackErr
	}
	return err
}
