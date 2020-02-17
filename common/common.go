package common

import (
	"time"
)

type Account interface {
	Login(id, password string, options map[string]interface{}) error // Internal use only. see: bankpkg.Login(...)
	Logout() error
	AccountInfo() *BankAccount
	TotalBalance() (int64, error)
	LastLogin() (time.Time, error)
	Recent() ([]*Transaction, error)
	History(from, to time.Time) ([]*Transaction, error)
	NewTransferToRegisteredAccount(targetName string, amount int64) (TransferState, error)
	CommitTransfer(tr TransferState, passwd string) (string, error)
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

type TransferState interface {
	Amount() int64
	FeeMessage() string
}
