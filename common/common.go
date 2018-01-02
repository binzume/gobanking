package common

import (
	"time"
)

type Account interface {
	Login(id, password string, params interface{}) error
	Logout() error
	TotalBalance() (int64, error)
	Recent() ([]*Transaction, error)
	History(from, to time.Time) ([]*Transaction, error)
	// NewTransactionWithNick(targetName string, amount int) (TempTransaction, error)
}

type Transaction struct {
	Date        time.Time
	Amount      int64
	Description string
}

type TempTransaction interface {
	Amount() int64
	Fee() int
	FeeMessage() string
	Commit(params interface{}) (string, error)
}
