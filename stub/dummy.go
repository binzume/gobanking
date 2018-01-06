package stub

import (
	"time"

	"github.com/binzume/go-banking/common"
)

type Account struct {
	common.BankAccount
	balance int64
}

const BankCode = "9999"
const BankName = "テスト銀行"

var _ common.Account = &Account{}

func Login(id, password string) (*Account, error) {
	a := &Account{
		common.BankAccount{BankCode: BankCode, BankName: BankName, BranchCode: "001", BranchName: "テスト支店", OwnerName: id},
		123456712345678,
	}
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
	return a.balance, nil
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
func (a *Account) NewTransactionWithNick(targetName string, amount int64) (common.TempTransaction, error) {
	return common.TempTransactionMap{"fee": 100, "amount": amount, "to": targetName}, nil
}

func (a *Account) CommitTransaction(tr common.TempTransaction, pass2 string) (string, error) {
	return "dummy", nil
}
