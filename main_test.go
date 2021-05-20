package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
)

// PotentialGuest is a struct to capture a guest from request before he added to guest list
type PotentialGuest struct {
	Table               int `json:"table"`
	AccompaniyingGuests int `json:"accompanying_guests"`
}

// Constants of a guest to add him to the guest list
const (
	ACCOMPANYING_GUESTS = 3
	NAME                = "john"
	TABLE               = 2
)

// TestReturnGuestlist test that verifies that ReturnGuestlist request
// utilizes the right request method, route and checks if response is correct
func TestReturnGuestlist(t *testing.T) {
	ts := getReturnGuestlistTestServer(t)
	err := makeReturnGuestlistRequest(ts.URL, t)
	if err != nil {
		t.Errorf("makeReturnGuestlistRequest() returned an error: %s", err)
	}
	defer ts.Close()
}

// TestAddGuestToGuestlist test that verifies that AddGuestToGuestlist request
// utilizes the right request method, route and checks if response is correct
func TestAddGuestToGuestlist(t *testing.T) {
	ts := getAddGuestToGuestlistTestServer(t)
	err := makeAddToGuestBookRequest(ts.URL, t)
	if err != nil {
		t.Errorf("makeAddToGuestBookRequest() returned an error: %s", err)
	}
	defer ts.Close()
}

// TestGetArrivedGuests test that verifies that GetArrivedGuests request
// utilizes the right request method, route and checks if response is correct
func TestGetArrivedGuests(t *testing.T) {
	ts := getGetArrivedGuestsTestServer(t)
	err := makeGetArrivedGuestsRequest(ts.URL, t)
	if err != nil {
		t.Errorf("makeGetArrivedGuestsRequest() returned an error: %s", err)
	}
	defer ts.Close()
}

// getReturnGuestlistTestServer establishes a test http server with its predefined response
func getReturnGuestlistTestServer(t *testing.T) *httptest.Server {
	ts := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			if r.Method != "GET" {
				t.Errorf(`Expected GET request, got ‘%s’`, r.Method)
			}
			if r.URL.EscapedPath() != "/guest_list" {
				t.Errorf(`Expected request to ‘/guest_list, got ‘%s’`, r.URL.EscapedPath())
			}

			guestlist := GuestList{
				[]Guest{
					{Name: "john", AccompaniyingGuests: 3, Table: 1},
					{Name: "mike", AccompaniyingGuests: 2, Table: 1},
					{Name: "johana", AccompaniyingGuests: 4, Table: 2},
					{Name: "greg", AccompaniyingGuests: 2, Table: 3},
				},
			}
			json.NewEncoder(w).Encode(guestlist)
		},
	))
	return ts
}

// getAddGuestToGuestlistTestServer establishes a test http server with its predefined response
func getAddGuestToGuestlistTestServer(t *testing.T) *httptest.Server {
	ts := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			if r.Method != "POST" {
				t.Errorf(`Expected POST request, got ‘%s’`, r.Method)
			}
			if r.URL.EscapedPath() != fmt.Sprintf("/guest_list/%s", NAME) {
				t.Errorf(`Expected request to ‘/guest_list/%s, got ‘%s’`, r.URL.EscapedPath(), NAME)
			}
			reqBody, _ := ioutil.ReadAll(r.Body)
			var guest PotentialGuest
			json.Unmarshal(reqBody, &guest)
			if guest.AccompaniyingGuests < 0 {
				t.Errorf(`Expected 'accompanying guests' be > 0, got ‘%v’`, guest.AccompaniyingGuests)
			}
			if guest.Table < 0 {
				t.Errorf(`Expected 'table' be > 0, got ‘%v’`, guest.Table)
			}
			person := Person{Name: NAME}
			json.NewEncoder(w).Encode(person)
		},
	))
	return ts
}

// makeReturnGuestlistRequest simulates a request to hit ReturnGuestlist endpoint
func makeReturnGuestlistRequest(testUrl string, t *testing.T) error {
	url := fmt.Sprintf("%s/guest_list", testUrl)
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf(`guestbook didn’t respond 200 OK: %s`, resp.Status)
	}

	body, _ := ioutil.ReadAll(resp.Body)
	var list GuestList
	json.Unmarshal(body, &list)
	if guest_size := len(list.Guests); guest_size != 4 {
		t.Errorf("Expected request to return 4 guests, got: ‘%v’", guest_size)
	}
	return nil
}

// makeAddToGuestBookRequest simulates a request to hit AddToGuestBook endpoint
func makeAddToGuestBookRequest(testUrl string, t *testing.T) error {
	url := fmt.Sprintf("%s/guest_list/%s", testUrl, NAME)
	guest := PotentialGuest{
		Table:               TABLE,
		AccompaniyingGuests: ACCOMPANYING_GUESTS,
	}
	json_data, err := json.Marshal(guest)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(json_data))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("guestbook didn’t respond 200 OK: %s", resp.Status)
	}

	body, _ := ioutil.ReadAll(resp.Body)
	var person Person
	json.Unmarshal(body, &person)
	if person.Name != NAME {
		t.Errorf("Expected request to return %s, got: ‘%s’", NAME, person.Name)
	}
	return nil
}

// getGetArrivedGuestsTestServer establishes a test http server with its predefined response
func getGetArrivedGuestsTestServer(t *testing.T) *httptest.Server {
	ts := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			if r.Method != "GET" {
				t.Errorf(`Expected GET request, got ‘%s’`, r.Method)
			}
			if r.URL.EscapedPath() != "/guests" {
				t.Errorf(`Expected request to ‘/guests, got ‘%s’`, r.URL.EscapedPath())
			}

			arrivedGuestlist := ArrivedGuestList{
				[]ArrivedGuest{
					{Name: "john", AccompaniyingGuests: 3, TimeArrived: "2021-05-20 16:32:32"},
					{Name: "mike", AccompaniyingGuests: 2, TimeArrived: "2021-05-20 16:33:33"},
					{Name: "johana", AccompaniyingGuests: 4, TimeArrived: "2021-05-20 16:39:00"},
					{Name: "greg", AccompaniyingGuests: 2, TimeArrived: "2021-05-20 14:23:23"},
				},
			}
			json.NewEncoder(w).Encode(arrivedGuestlist)
		},
	))
	return ts
}

// makeGetArrivedGuestsRequest simulates a request to hit GetArrivedGuests endpoint
func makeGetArrivedGuestsRequest(testUrl string, t *testing.T) error {
	url := fmt.Sprintf("%s/guests", testUrl)
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf(`guestbook didn’t respond 200 OK: %s`, resp.Status)
	}

	body, _ := ioutil.ReadAll(resp.Body)
	var list ArrivedGuestList
	json.Unmarshal(body, &list)
	if guest_size := len(list.Guests); guest_size != 4 {
		t.Errorf("Expected request to return 4 guests, got: ‘%v’", guest_size)
	}
	return nil
}
