package common

import (
	"time"
)

type Account interface {
	Logout() error
	TotalBalance() (int64, error)
	Recent() ([]*Transaction, error)
	History(from, to time.Time) ([]*Transaction, error)
}

type Transaction struct {
	Date        time.Time
	Amount      int64
	Description string
}
