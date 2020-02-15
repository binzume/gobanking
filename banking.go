package banking

import (
	"encoding/json"
	"errors"
	"io/ioutil"

	"github.com/binzume/gobanking/common"
	"github.com/binzume/gobanking/mizuho"
	"github.com/binzume/gobanking/rakuten"
	"github.com/binzume/gobanking/sbi"
	"github.com/binzume/gobanking/shinsei"
	"github.com/binzume/gobanking/stub"
)

type AccountConfig struct {
	Bank     string                 `json:"bank"`
	Id       string                 `json:"id"`
	Password string                 `json:"password"`
	Options  map[string]interface{} `json:"options"`
}

func LoginWithJsonFile(path string) (common.Account, error) {
	buf, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var c AccountConfig
	err = json.Unmarshal(buf, &c)
	if err != nil {
		return nil, err
	}
	return Login(&c)
}

func Login(c *AccountConfig) (common.Account, error) {
	switch c.Bank {
	case "mizuho":
		return mizuho.Login(c.Id, c.Password, c.Options)
	case "rakuten":
		return rakuten.Login(c.Id, c.Password, c.Options)
	case "shinsei":
		return shinsei.Login(c.Id, c.Password, c.Options)
	case "sbi":
		return sbi.Login(c.Id, c.Password, c.Options)
	case "stub":
		return stub.Login(c.Id, c.Password, c.Options)
	default:
		return nil, errors.New("unknown:" + c.Bank)
	}
}
