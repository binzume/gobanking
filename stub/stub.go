package stub

import (
	"errors"
	"time"

	"github.com/binzume/gobanking/common"
	"github.com/binzume/gobanking/utils"
)

type Account struct {
	common.BankAccount
	options map[string]interface{}
}

const BankCode = "9999"
const BankName = "テスト銀行"

func Login(id, password string, options map[string]interface{}) (*Account, error) {
	a := &Account{
		BankAccount: common.BankAccount{BankCode: BankCode, BankName: BankName, BranchCode: "001", BranchName: "テスト支店", OwnerName: id},
	}
	err := a.Login(id, password, options)
	return a, err
}

func (a *Account) Login(id, password string, options map[string]interface{}) error {
	if id == "" || password == "" {
		return errors.New("login error")
	}
	a.options = options
	return nil
}

func (a *Account) Logout() error {
	return nil
}

func (a *Account) AccountInfo() *common.BankAccount {
	return &a.BankAccount
}

func (a *Account) TotalBalance() (int64, error) {
	return a.getInt("balance", 1234567), nil
}

func (a *Account) LastLogin() (time.Time, error) {
	return time.Now(), nil
}

func (a *Account) Recent() ([]*common.Transaction, error) {
	base := time.Now().Truncate(time.Hour * 24).Add(-time.Hour * 24 * 7) // week ago today.
	return []*common.Transaction{
		&common.Transaction{Date: base, Amount: 123, Balance: 123, Description: "test"},
		&common.Transaction{Date: base.Add(time.Hour * 48), Amount: 10000, Balance: 10123, Description: "test2"},
		&common.Transaction{Date: time.Now().Truncate(time.Second), Amount: -5000, Balance: 5123, Description: "test..."},
	}, nil
}

func (a *Account) History(from, to time.Time) ([]*common.Transaction, error) {
	return a.Recent()
}

// transfar api
func (a *Account) NewTransferToRegisteredAccount(targetName string, amount int64) (common.TransferState, error) {
	if amount == 0 || targetName == "" {
		return nil, errors.New("transfer error")
	}
	return utils.TransferStateMap{"fee": a.getInt("transfer_fee", 100), "amount": amount, "to": targetName}, nil
}

func (a *Account) CommitTransfer(tr common.TransferState, pass2 string) (string, error) {
	if pass2 == "" {
		return "", errors.New("commit error")
	}
	return "dummy", nil
}

func (a *Account) getInt(name string, defvalue int64) int64 {
	if a.options != nil {
		if v, ok := a.options[name]; ok {
			if i, ok := v.(int); ok {
				return int64(i)
			} else if i, ok := v.(float64); ok {
				return int64(i)
			}
		}
	}
	return defvalue
}
