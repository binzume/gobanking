// +build ignore

package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"

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
		return stub.Login(c.Id, c.Password, c.Options)
	default:
		return nil, errors.New("unknown:" + c.Bank)
	}
}

func main() {
	if len(os.Args) < 4 {
		fmt.Println(" Usage: go run trans.go accounts/stub.json targetname 10000")
		return
	}
	target := os.Args[2]
	amount, err := strconv.Atoi(os.Args[3])
	if err != nil {
		fmt.Println(" Usage: go run trans.go accounts/stub.json targetname 10000")
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
		fmt.Println("LastLogin() error.", err)
	}
	fmt.Println("Last Login:", lastLogin)

	// Print balance.
	total, err := acc.TotalBalance()
	if err != nil {
		fmt.Println("TotalBalance() error.", err)
	}
	fmt.Println("Balance:", total)

	tr, err := acc.NewTransactionWithNick(target, int64(amount))
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(tr)

	fmt.Print("Enter pass: ")
	reader := bufio.NewReader(os.Stdin)
	pass, _ := reader.ReadString('\n')
	pass = strings.TrimSpace(pass)
	if pass == "" {
		fmt.Println("canceled")
		return
	}

	recptId, err := acc.CommitTransaction(tr, pass)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("ok. recptId:", recptId)
}
