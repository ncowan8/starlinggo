package starlingGo

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

type Accounter interface {
	getDirectDebits()
	getRecurringPayments()
	getStandingOrders()
	getBalance()
	getTransactionsSince()
	leftToPayReport()
	getLastPayDay()
}

type Account struct {
	Token       string
	AccountUid  string
	CategoryUid string
}

func (x Account) get(endpoint string) []byte {

	client := &http.Client{}
	req, _ := http.NewRequest("GET", BaseUrl+endpoint, nil)

	req.Header.Set("Authorization", "Bearer "+x.Token)

	resp, err := client.Do(req)
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

func (x Account) getDirectDebits() []directDebit {

	resp := x.get("direct-debit/mandates")
	var dds directDebits
	if err := json.Unmarshal(resp, &dds); err != nil {
		log.Fatal(err)
	}
	return dds.Mandates
}

func (x Account) getRecurringPayments() []recurringPayment {
	resp := x.get(fmt.Sprintf("accounts/%s/recurring-payment", x.AccountUid))
	var rps recurringPayments
	if err := json.Unmarshal(resp, &rps); err != nil {
		log.Fatal(err)
	}
	return rps.RecurringPayments
}

func (x Account) getStandingOrders() []standingOrder {
	resp := x.get(fmt.Sprintf("payments/local/account/%s/category/%s/standing-orders", x.AccountUid, x.CategoryUid))
	var sos standingOrders
	if err := json.Unmarshal(resp, &sos); err != nil {
		log.Fatal(err)
	}
	return sos.StandingOrders
}

func (x Account) getBalance() float64 {

	resp := x.get(fmt.Sprintf("accounts/%s/balance", x.AccountUid))
	var bal balances
	if err := json.Unmarshal(resp, &bal); err != nil {
		log.Fatal(err)
	}
	return bal.EffectiveBalance.Pence / 100.00

}

func (x Account) getTransactionsSince(since time.Time) transactions {

	resp := x.get(fmt.Sprintf("feed/account/%s/category/%s?changesSince=%s", x.AccountUid, x.CategoryUid, since.Format(datetimeFmt)))
	var t transactions
	if err := json.Unmarshal(resp, &t); err != nil {
		log.Fatal(err)
	}
	return t
}

func getLastPayDay(ts []transaction, ref string, employer string) time.Time {
	var dt time.Time
	for _, t := range ts {
		if t.Direction == "IN" && t.Reference == ref && t.PartyName == employer {
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
			report += writeRepTab(e.Status, e.Payee, e.LatestPayment.LastAmount.Pence/100.00, dt.Format(dateFmt))
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

func (x Account) Report(payRef string, employer string) string {

	now := time.Now()

	since := now.AddDate(0, -1, 0)
	trans := x.getTransactionsSince(since).Transactions
	payDay := getLastPayDay(trans, payRef, employer)
	dd, so, rp, bal := x.getDirectDebits(), x.getStandingOrders(), x.getRecurringPayments(), x.getBalance()
	toPay, toPayRep := leftToPayReport(dd, rp, so, payDay)
	report := fmt.Sprintf("<head><style>table, th, td {border: 1px solid black;}</style></head><body><table>%s</table>", toPayRep)
	// add invisable uncode char to force utf-8 as gmail is dumb
	report += fmt.Sprintf("<p>\u200BLast pay day %s</p>", payDay.Format(dateFmt))
	report += fmt.Sprintf("<p><strong>Balance £%6.2f - To Pay £%6.2f - Remaining balance £%6.2f</strong></p></body>", bal, toPay, bal-toPay)
	return report

}
