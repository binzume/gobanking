package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	// TODO replace .. to github.com/binzume/go-banking
	"../mizuho"
	"../rakuten"
	"../sbi"
	"../shinsei"
	"../stub"
	"github.com/binzume/go-banking/common"
)

type AccountConfig struct {
	Bank     string                 `json:"bank"`
	Id       string                 `json:"id"`
	Password string                 `json:"password"`
	Options  map[string]interface{} `json:"options"`
}

func login(path string) (common.Account, error) {
	raw, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var c AccountConfig
	err = json.Unmarshal(raw, &c)
	if err != nil {
		return nil, err
	}
	switch c.Bank {
	case "mizuho":
		words := map[string]string{}
		for k, v := range c.Options {
			words[k] = v.(string)
		}
		return mizuho.Login(c.Id, c.Password, words)
	case "rakuten":
		words := map[string]string{}
		for k, v := range c.Options {
			words[k] = v.(string)
		}
		return rakuten.Login(c.Id, c.Password, words)
	case "shinsei":
		grid := []string{}
		for _, f := range c.Options["grid"].([]interface{}) {
			grid = append(grid, f.(string))
		}
		return shinsei.Login(c.Id, c.Password, c.Options["numId"].(string), grid)
	case "sbi":
		return sbi.Login(c.Id, c.Password)
	case "stub":
		return stub.Login(c.Id, c.Password)
	default:
		return nil, errors.New("unknown:" + c.Bank)
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println(" Usage: go run account_info.go accounts/stub.json")
		return
	}

	// Login with json file.
	acc, err := login(os.Args[1])
	if err != nil {
		fmt.Println("Login error.", err)
		return
	}
	defer acc.Logout()

	lastLogin, err := acc.LastLogin()
	if err != nil {
		fmt.Println("LastLogin error.", err)
	}
	fmt.Println("Last Login:", lastLogin)

	// Print balance.
	total, err := acc.TotalBalance()
	if err != nil {
		fmt.Println("TotalBalance error.", err)
	}
	fmt.Println("Balance:", total)

	// Print recent logs.
	trs, err := acc.Recent()
	if err != nil {
		fmt.Println("Get recent history error.", err)
	}
	fmt.Println("--------")
	for _, tr := range trs {
		fmt.Println(tr.Date, tr.Amount, tr.Description, tr.Balance)
	}
	fmt.Println("--------")

	// Print History.
	/*
		trs, err = acc.History(time.Now().Add(-time.Hour*24*60), time.Now().Add(-time.Hour*24*20))
		if err != nil {
			log.Fatal(err)
		}
		for _, tr := range trs {
			fmt.Println(tr.Date, tr.Amount, tr.Description, tr.Balance)
		}
	*/
}
