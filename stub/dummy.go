package stub

import (
	"time"

	"../common"
)

type Account struct {
}

type TempTransaction map[string]interface{}

const BankCode = "9999"

var _ common.Account = &Account{}

func Login(id, password string) (*Account, error) {
	a := &Account{}
	err := a.Login(id, password, nil)
	return a, err
}

func (a *Account) Login(id, password string, params interface{}) error {
	return nil
}

func (a *Account) Logout() error {
	return nil
}

func (a *Account) TotalBalance() (int64, error) {
	return 0, nil
}

func (a *Account) LastLogin() (time.Time, error) {
	return time.Now(), nil
}

func (a *Account) Recent() ([]*common.Transaction, error) {
	return nil, nil
}

func (a *Account) History(from, to time.Time) ([]*common.Transaction, error) {
	return nil, nil
}

// transfar api
func (a *Account) NewTransactionWithNick(targetName string, amount int) (TempTransaction, error) {
	return nil, nil
}

func (a *Account) Commit(tr TempTransaction, pass2 string) (string, error) {
	return "dummy", nil
}
