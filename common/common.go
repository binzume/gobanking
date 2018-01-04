package common

import (
	"time"
)

type Account interface {
	Login(id, password string, params interface{}) error
	Logout() error
	TotalBalance() (int64, error)
	LastLogin() (time.Time, error)
	Recent() ([]*Transaction, error)
	History(from, to time.Time) ([]*Transaction, error)
	NewTransactionWithNick(targetName string, amount int64) (TempTransaction, error)
	CommitTransaction(tr TempTransaction, passwd string) (string, error)
}

type BankAccount struct {
	BankName   string
	BankCode   string
	BranchName string
	BranchCode string
	AccountNum string
	OwnerName  string
}

type Transaction struct {
	Date        time.Time
	Amount      int64
	Balance     int64
	Description string
}

type TempTransaction interface {
	Amount() int64
	FeeMessage() string
}
