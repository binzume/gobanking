package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"

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

	acc, err := login("accounts/stub.json")
	if err != nil {
		log.Fatal(err)
	}
	defer acc.Logout()

	log.Println(acc.LastLogin())

	// Print balance.
	log.Println(acc.TotalBalance())

	// Print recent logs.
	trs, err := acc.Recent()
	if err != nil {
		log.Fatal(err)
	}
	for _, tr := range trs {
		log.Println(tr.Date, tr.Amount, tr.Description, tr.Balance)
	}

	// TODO
	/*
		trs, err = acc.History(time.Now().Add(-time.Hour*24*60), time.Now().Add(-time.Hour*24*20))
		if err != nil {
			log.Fatal(err)
		}
		for _, tr := range trs {
			log.Println(tr.Date, tr.Amount, tr.Description, tr.Balance)
		}
	//*/
	/*
		tr, err := acc.NewTransactionWithNick("test", 1500000)
		if err != nil {
			log.Fatal(err)
		}
		log.Println(tr)
	//*/
	/*
		recptNo, err := acc.CommitTransaction(tr, "00000000")
		if err != nil {
			log.Fatal(err)
		}
		log.Println("recptNo", recptNo)
	//*/

}
