package starlinggo

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"
)

var testToken = "bob"
var testAccountUid = "abcd"
var testCategoryUid = "efgh"

// MockClient is the mock client
type MockClient struct {
	DoFunc func(req *http.Request) (*http.Response, error)
}

var (
	// GetDoFunc fetches the mock client's `Do` func
	GetDoFunc func(req *http.Request) (*http.Response, error)
)

// Do is the mock client's `Do` func
func (m *MockClient) Do(req *http.Request) (*http.Response, error) {
	return GetDoFunc(req)
}

func init() {

	Client = &MockClient{}
}

// manually creation an account for use in tests
var testAccount = Account{Token: testToken, AccountUid: testAccountUid, CategoryUid: testCategoryUid}

func Test_writeRepTab(t *testing.T) {
	// token test function
	s := "s"
	p := "p"
	a := 1.00
	d := "d"
	result := writeRepTab(s, p, a, d)

	// check only 1 space between the words
	want := fmt.Sprintf("<tr><td>%-20s</td><td>%-30s</td><td>£%8.2f</td><td>%s</td></tr>", s, p, a, d)
	if want != result {
		t.Fatalf(`result = %q want match for %#q`, result, want)
	}

}

func Test_AccountInit(t *testing.T) {

	json := `{
	"accounts": [
	  {
		"accountUid": "correct",
		"accountType": "PRIMARY",
		"defaultCategory": "correct",
		"currency": "GBP",
		"createdAt": "2022-04-17T19:46:17.663Z",
		"name": "Personal"
	  },
	  {
		"accountUid": "incorrect",
		"accountType": "SECONDAY",
		"defaultCategory": "incorrect",
		"currency": "GBP",
		"createdAt": "2022-04-17T19:46:17.663Z",
		"name": "Joint"
	  }
	]
}`

	// create a new reader with that JSON
	GetDoFunc = func(*http.Request) (*http.Response, error) {
		r := ioutil.NopCloser(bytes.NewReader([]byte(json)))
		return &http.Response{
			StatusCode: 200,
			Body:       r,
		}, nil
	}

	var testAccount = AccountInit(testToken)
	if testAccount.Token != testToken {
		t.Fatalf(`result = %q want match for %#q`, testAccount.Token, testToken)
	}

	if testAccount.AccountUid != "correct" {
		t.Fatalf(`result = %q want match for %#q`, testAccount.AccountUid, "correct")
	}

	if testAccount.CategoryUid != "correct" {
		t.Fatalf(`result = %q want match for %#q`, testAccount.CategoryUid, "correct")
	}

}

func Test_get(t *testing.T) {

	// build response JSON
	json := `{"name":"Test Name","full_name":"test full name","owner":{"login": "octocat"}}`
	// create a new reader with that JSON
	r := ioutil.NopCloser(bytes.NewReader([]byte(json)))

	GetDoFunc = func(h *http.Request) (*http.Response, error) {

		if h.Header.Get("Authorization") != "Bearer "+testToken {
			t.Fatalf(`result = %q want match for %#q`, h.Header.Get("Authorization"), "Bearer "+testToken)
		}

		return &http.Response{
			StatusCode: 200,
			Body:       r,
		}, nil
	}

	if result := get("anything", testToken); string(result) != json {
		t.Fatal("Uh oh stinky")
	}
}
