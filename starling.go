package starlinggo

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

var BaseUrl = "https://api.starlingbank.com/api/v2/"
var datetimeFmt = "2006-01-02T15:04:05.000Z"
var dateFmt = "2006-01-02"

type amount struct {
	Currency string  `json:"curency"`
	Pence    float64 `json:"minorUnits"`
}

type latestDDPayment struct {
	LastAmount amount `json:"lastAmount"`
	LastDate   string `json:"lastDate"`
}

type directDebit struct {
	Status        string          `json:"status"`
	Payee         string          `json:"originatorName"`
	LatestPayment latestDDPayment `json:"lastPayment"`
}

type directDebits struct {
	Mandates []directDebit `json:"mandates"`
}

type recurringPayment struct {
	Payee      string `json:"counterPartyName"`
	Status     string `json:"status"`
	LastAmount amount `json:"latestPaymentAmount"`
	LastDate   string `json:"latestPaymentDate"`
}

type recurringPayments struct {
	RecurringPayments []recurringPayment `json:"recurringPayments"`
}

type standingOrder struct {
	Reference  string `json:"reference"`
	Amount     amount `json:"amount"`
	CancelDate string `json:"cancelledAt"`
	NextDate   string `json:"nextdate"`
}

type standingOrders struct {
	StandingOrders []standingOrder `json:"standingOrders"`
}

type balances struct {
	EffectiveBalance amount `json:"effectiveBalance"`
	ClearedBalance   amount `json:"clearedBalance"`
	Amount           amount `json:"amount"`
}
type transaction struct {
	Amount        amount `json:"amount"`
	Direction     string `json:"direction"`
	TransactionDt string `json:"transactionTime"`
	PartyName     string `json:"counterPartyName"`
	Reference     string `json:"reference"`
}

type transactions struct {
	Transactions []transaction `json:"feedItems"`
}

type accounts struct {
	Accounts []accountDetial `json:"accounts"`
}

type accountDetial struct {
	AccountUid  string `json:"accountUid"`
	AccountType string `json:"accountType"`
	CategoryUid string `json:"defaultCategory"`
	Name        string `json:"name"`
}

// type Accounter interface {
// 	getDirectDebits()
// 	getRecurringPayments()
// 	getStandingOrders()
// 	GetBalance()
// 	getTransactionsSince()
// 	leftToPayReport()
// 	getLastPayDay()
// }

type Account struct {
	Token, AccountUid, CategoryUid string
}

// HTTPClient interface
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

var Client HTTPClient

func init() {
	Client = &http.Client{}
}

// Initialise and return an account
func AccountInit(token string) Account {

	au, cu := getPrimaryAccountDetails(token)
	acc := Account{Token: token, AccountUid: au, CategoryUid: cu}
	return acc

}

// Generic get function that makes the request to starling api
func get(endpoint string, token string) []byte {

	req, _ := http.NewRequest("GET", BaseUrl+endpoint, nil)

	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := Client.Do(req)
	if err != nil {
		log.Fatalln(err)
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}

	return body
}

// Function to gather additional account info for intialisation
func getPrimaryAccountDetails(token string) (string, string) {

	resp := get("accounts", token)
	var accs accounts
	fmt.Println(string(resp))
	if err := json.Unmarshal(resp, &accs); err != nil {
		log.Fatal(err.Error())
	}

	var accUid, catUid string
	for _, acc := range accs.Accounts {
		if acc.AccountType == "PRIMARY" {
			accUid, catUid = acc.AccountUid, acc.CategoryUid
		}
	}
	return accUid, catUid
}

// Function to collect a list of all active direct debits for the account
func (x Account) getDirectDebits() []directDebit {

	resp := get("direct-debit/mandates", x.Token)
	var dds directDebits
	if err := json.Unmarshal(resp, &dds); err != nil {
		log.Fatal(err)
	}
	return dds.Mandates
}

func (x Account) getRecurringPayments() []recurringPayment {
	resp := get(fmt.Sprintf("accounts/%s/recurring-payment", x.AccountUid), x.Token)
	var rps recurringPayments
	if err := json.Unmarshal(resp, &rps); err != nil {
		log.Fatal(err)
	}
	return rps.RecurringPayments
}

func (x Account) getStandingOrders() []standingOrder {
	resp := get(fmt.Sprintf("payments/local/account/%s/category/%s/standing-orders", x.AccountUid, x.CategoryUid), x.Token)
	var sos standingOrders
	if err := json.Unmarshal(resp, &sos); err != nil {
		log.Fatal(err)
	}
	return sos.StandingOrders
}

// Function to return the effective balance of the account in pounds
func (x Account) GetBalance() float64 {

	resp := get(fmt.Sprintf("accounts/%s/balance", x.AccountUid), x.Token)
	var bal balances
	if err := json.Unmarshal(resp, &bal); err != nil {
		log.Fatal(err)
	}
	return bal.EffectiveBalance.Pence / 100.00

}

func (x Account) getTransactionsSince(since time.Time) transactions {

	resp := get(fmt.Sprintf("feed/account/%s/category/%s?changesSince=%s", x.AccountUid, x.CategoryUid, since.Format(datetimeFmt)), x.Token)
	var t transactions
	if err := json.Unmarshal(resp, &t); err != nil {
		log.Fatal(err)
	}
	return t
}

func getLastPayDay(ts []transaction, ref string) time.Time {
	var dt time.Time
	for _, t := range ts {
		if t.Direction == "IN" && t.Reference == ref {
			dt, _ = time.Parse(datetimeFmt, t.TransactionDt)
		}
	}
	return dt
}

func writeRepTab(status string, payee string, amnt float64, dt string) string {
	return fmt.Sprintf("<tr><td>%-20s</td><td>%-30s</td><td>£%8.2f</td><td>%s</td></tr>", status, payee, amnt, dt)
}

func leftToPayReport(dd []directDebit, rp []recurringPayment, so []standingOrder, payDate time.Time) (float64, string) {

	total := 0.00
	report := ""
	for _, e := range dd {
		dt, _ := time.Parse(dateFmt, e.LatestPayment.LastDate)
		if e.Status == "LIVE" && dt.Before(payDate) {
			report += writeRepTab("ACTIVE", e.Payee, e.LatestPayment.LastAmount.Pence/100.00, dt.Format(dateFmt))
			total += e.LatestPayment.LastAmount.Pence
		}
	}

	for _, e := range rp {

		dt, _ := time.Parse(datetimeFmt, e.LastDate)
		if e.Status == "ACTIVE" && dt.Before(payDate) {
			report += writeRepTab(e.Status, e.Payee, e.LastAmount.Pence/100.00, dt.Format(dateFmt))
			total += e.LastAmount.Pence
		}
	}

	for _, e := range so {

		dt, _ := time.Parse(datetimeFmt, e.NextDate)
		if e.CancelDate != "" && dt.Before(payDate.AddDate(0, 1, 0)) && dt.After(payDate) {
			report += writeRepTab("ACTIVE", e.Reference, e.Amount.Pence/100.00, dt.Format(dateFmt))
			total += e.Amount.Pence
		}
	}

	return float64(total) / 100.00, report

}

func (x Account) Report(payRef string) string {

	now := time.Now()
	since := now.AddDate(0, -1, 0)
	trans := x.getTransactionsSince(since).Transactions
	payDay := getLastPayDay(trans, payRef)
	dd, so, rp, bal := x.getDirectDebits(), x.getStandingOrders(), x.getRecurringPayments(), x.GetBalance()
	toPay, toPayRep := leftToPayReport(dd, rp, so, payDay)
	report := fmt.Sprintf("<head><style>table, th, td {border: 1px solid black;}</style></head><body><table>%s</table>", toPayRep)
	// add invisable uncode char to force utf-8 as gmail is dumb
	report += fmt.Sprintf("<p>\u200BLast pay day %s</p>", payDay.Format(dateFmt))
	report += fmt.Sprintf("<p>Balance £%6.2f<br>To Pay £%6.2f<br>Effective balance £%6.2f</p></body>", bal, toPay, bal-toPay)
	return report

}
