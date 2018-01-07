package common

import (
	"time"
)

type Account interface {
	Login(id, password string, params interface{}) error // Use: bankpkg.Login(...)
	Logout() error
	TotalBalance() (int64, error)
	LastLogin() (time.Time, error)
	Recent() ([]*Transaction, error)
	History(from, to time.Time) ([]*Transaction, error)
	NewTransactionWithNick(targetName string, amount int64) (TempTransaction, error) // TODO: Rename
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
	Date        time.Time `json:"date"`
	Amount      int64     `json:"amount"`
	Balance     int64     `json:"balance"`
	Description string    `json:"description"`
}

type TempTransaction interface {
	Amount() int64
	FeeMessage() string
}
