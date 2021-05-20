package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"text/template"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux"
)

var tmpl = template.Must(template.ParseGlob("form/*"))

// Person is struct that charaterizes arrived and left guests by name
type Person struct {
	Name string `json:"name"`
}

// ArrivedGuest is sturct that helps to process arriving guest
type ArrivedGuest struct {
	Name                string `json:"name"`
	AccompaniyingGuests int    `json:"accompanying_guests"`
	TimeArrived         string `json:"time_arrived"`
}

// Guest is struct that characterizes guest on a reserve
type Guest struct {
	Name                string `json:"name"`
	Table               int    `json:"table"`
	AccompaniyingGuests int    `json:"accompanying_guests"`
}

// ArrivedGuestList is struct that helps to output arrived guests
type ArrivedGuestList struct {
	Guests []ArrivedGuest `json:"guests"`
}

// GuestList is struct that helps to output guest list
type GuestList struct {
	Guests []Guest `json:"guests"`
}

// dbConn returns a mysql database reference to guestbookdb
func dbConn() (db *sql.DB) {
	dbDriver := "mysql"
	dbUser := "root"
	dbPass := "admin123"
	dbName := "guestbookdb"
	db, err := sql.Open(dbDriver, dbUser+":"+dbPass+"@/"+dbName)
	if err != nil {
		panic(err.Error())
	}
	return db
}

// CountSeatsEmpty is handler that returns number of empty seats as a response
func CountSeatsEmpty(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Endpoint hit: CountSeatsEmpty")

	db := dbConn()
	selectDB, err := db.Query("select sum(capacity - num_occupied) as seats_empty from tables")
	if err != nil {
		panic(err.Error())
	}
	response := struct {
		SeatsEmpty int `json:"seats_empty"`
	}{
		SeatsEmpty: 0,
	}

	for selectDB.Next() {
		var seats_empty int

		err = selectDB.Scan(
			&seats_empty,
		)
		response.SeatsEmpty = seats_empty

		if err != nil {
			panic(err.Error())
		}
	}
	//json.NewEncoder(w).Encode(response)
	defer db.Close()
	tmpl.ExecuteTemplate(w, "SeatsEmpty", response)
}

// GetArrivedGuests is a handler that returns a list of arrived guests as a response
func GetArrivedGuests(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Endpoint hit: GetArrivedGuests")

	db := dbConn()
	selectDB, err := db.Query("SELECT name, num_arrived, time_arrived FROM guests WHERE num_arrived > 0 ORDER BY name")
	if err != nil {
		panic(err.Error())
	}
	guest := ArrivedGuest{}
	guest_list := ArrivedGuestList{}
	for selectDB.Next() {
		var num_arrived int
		var name, time_arrived string

		err = selectDB.Scan(
			&name,
			&num_arrived,
			&time_arrived,
		)
		guest.Name = name
		guest.AccompaniyingGuests = num_arrived - 1
		guest.TimeArrived = time_arrived

		if err != nil {
			panic(err.Error())
		}
		guest_list.Guests = append(guest_list.Guests, guest)
	}
	//json.NewEncoder(w).Encode(guest_list)
	tmpl.ExecuteTemplate(w, "Arrived", guest_list.Guests)
	defer db.Close()
}

// RegisterArrivedGuest is a handler that processes ariving guests
// It throws error if guest is not on on the guest list or
// if the table cannot accomodate extra entourage
func RegisterArrivedGuest(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Endpoint hit: RegisterArrivedGuest")

	vars := mux.Vars(r)
	name := vars["name"]
	acc_guests_str := r.FormValue("accompanying_guests")
	acc_guests_new, err := strconv.Atoi(acc_guests_str)

	if err != nil {
		panic(err.Error())
	}

	haveReservation, table, acc_guests_old, num_arrived := getGuestReservationInfo(name)
	if !haveReservation {
		panic(errors.New("Not reservation under the name"))
	}
	if num_arrived > 0 {
		panic(errors.New("Guest seems to arrive"))
	}

	canAccomodate, totalReservations, totalOccupations :=
		checkIfCanAccomodateExtra(table, acc_guests_old, acc_guests_new)
	if !canAccomodate {
		panic(errors.New("Not enough seats to accomodate extras"))
	}
	time_arrived := time.Now()
	updateGuestsArrival(name, acc_guests_new+1, time_arrived.Format("2006-01-02 15:04:05"))
	updateTableReservation(table, totalReservations)
	updateTableOccupation(table, totalOccupations)

	var person Person
	person.Name = name
	json.NewEncoder(w).Encode(person)
}

// RegisterLeftGuest is a handler that processes leaving guests
// It throws error if guest is not on a guest list or hasn't arrived yet
func RegisterLeftGuest(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Endpoint hit: RegisterLeftGuest")

	vars := mux.Vars(r)
	name := vars["name"]

	haveReservation, table, _, num_arrived := getGuestReservationInfo(name)
	if !haveReservation {
		panic(errors.New("Not reservation under the name"))
	}

	if num_arrived == 0 {
		panic(errors.New("Guest seems not arrived"))
	}

	gotSuchTable, _, num_occupied := getTableReservationInfo(table)
	if !gotSuchTable {
		panic(errors.New("No such table"))
	}

	updateGuestsArrival(name, 0, "")
	updateTableOccupation(table, num_occupied-num_arrived)
}

// getGuestReservationInfo retrieves guest reservation info from guests table
func getGuestReservationInfo(name string) (bool, int, int, int) {
	db := dbConn()
	var table_id int = -1
	var num_arrived = 0
	var accompanying_guests int
	selectDB, err := db.Query("SELECT table_id, accompanying_guests, num_arrived FROM guests WHERE name = ?", name)
	if err != nil {
		panic(err.Error())
	}
	for selectDB.Next() {
		err = selectDB.Scan(&table_id, &accompanying_guests, &num_arrived)
		if err != nil {
			panic(err.Error())
		}
	}
	if table_id == -1 {
		return false, 0, 0, 0
	}
	defer db.Close()
	return true, table_id, accompanying_guests, num_arrived
}

// AddGuestToGuestlist is a handler that adds guests to the guest list
// It throws error if guest is already on the guest list or there is not
// enough seats on the table left to reserve
func AddGuestToGuestlist(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Endpoint hit: AddGuestToGuestlist")

	vars := mux.Vars(r)
	name := vars["name"]
	table_str := r.FormValue("table")
	table, err := strconv.Atoi(table_str)
	if err != nil {
		panic(err.Error())
	}

	acc_guests_str := r.FormValue("accompanying_guests")
	accompanying_guests, err := strconv.Atoi(acc_guests_str)
	if err != nil {
		panic(err.Error())
	}

	if err != nil {
		panic(err.Error())
	}
	guest := Guest{Name: name, Table: table, AccompaniyingGuests: accompanying_guests}

	hasReservation, _, _, _ := getGuestReservationInfo(guest.Name)
	if hasReservation {
		panic(errors.New("Guest already has reservation"))
	}

	canReserve, totalReservations := checkIfCanReserveTable(guest.Table, guest.AccompaniyingGuests+1)
	if !canReserve {
		panic(errors.New("Not enough seats to reserve"))
	}

	insertGuest(guest)
	updateTableReservation(guest.Table, totalReservations)

	var person Person
	person.Name = name

	http.Redirect(w, r, "/guest_list", 301)
}

// checkIfCanReserveTable checks if the guest with entourage can be reserved on the table
func checkIfCanReserveTable(table int, guestsNum int) (bool, int) {
	db := dbConn()
	selectDB, err := db.Query("SELECT num_reserved, capacity FROM tables where table_id = ?", table)
	if err != nil {
		panic(err.Error())
	}
	var totalReservations int
	for selectDB.Next() {
		var num_reserved, capacity int

		err = selectDB.Scan(&num_reserved, &capacity)
		if err != nil {
			panic(err.Error())
		}
		totalReservations = num_reserved + guestsNum
		if totalReservations > capacity {
			return false, 0
		}
	}
	defer db.Close()
	return true, totalReservations
}

// checkIfCanAccomodateExtra checks if arriving guest with his entourage can be accomodated to the table
func checkIfCanAccomodateExtra(table int, acc_guests_old int, acc_guests_new int) (bool, int, int) {
	db := dbConn()
	selectDB, err := db.Query("SELECT num_reserved, num_occupied, capacity FROM tables where table_id = ?", table)
	if err != nil {
		panic(err.Error())
	}
	var totalReservations, totalOccupations int
	extra := acc_guests_new - acc_guests_old

	for selectDB.Next() {
		var num_reserved, num_occupied, capacity int

		err = selectDB.Scan(&num_reserved, &num_occupied, &capacity)
		if err != nil {
			panic(err.Error())
		}
		totalReservations = num_reserved + extra
		totalOccupations = num_occupied + acc_guests_new + 1
		if totalReservations > capacity {
			return false, 0, 0
		}
	}

	defer db.Close()
	return true, totalReservations, totalOccupations
}

// insertGuest add guest to the guests table
func insertGuest(guest Guest) {
	db := dbConn()
	insertForm, err := db.Prepare("INSERT INTO guests(name, accompanying_guests, table_id) VALUES(?,?,?)")
	if err != nil {
		panic(err.Error())
	}
	insertForm.Exec(guest.Name, guest.AccompaniyingGuests, guest.Table)

	log.Println("INSERT: Name: " +
		guest.Name + " | Accompanyig guests: " +
		strconv.Itoa(guest.AccompaniyingGuests) + " | Table: " +
		strconv.Itoa(guest.Table))
	defer db.Close()
}

// getNullString wraps an empty time string to a struct so that it could be inserted to the sql table as NULL
func getNullString(s string) sql.NullString {
	if len(s) == 0 {
		return sql.NullString{}
	}
	return sql.NullString{
		String: s,
		Valid:  true,
	}
}

// updateGuestsArrival updates guests table when guests arrives
func updateGuestsArrival(name string, numArrived int, timeArrivedStr string) {
	db := dbConn()
	insertForm, err := db.Prepare("UPDATE guests SET num_arrived=?, time_arrived= ? WHERE name=?")
	if err != nil {
		panic(err.Error())
	}
	timeArrived := getNullString(timeArrivedStr)
	insertForm.Exec(numArrived, timeArrived, name)

	log.Println("UPDATE: guests: " +
		name + " | num_arrived: " + strconv.Itoa(numArrived) + " | time_arrived: " + timeArrived.String)
	defer db.Close()
}

// updateTableReservation updates tables table when guest is added to the guest list and when he arrives
func updateTableReservation(table int, totalReservations int) {
	db := dbConn()
	insertForm, err := db.Prepare("UPDATE tables SET num_reserved=? WHERE table_id=?")
	if err != nil {
		panic(err.Error())
	}
	insertForm.Exec(totalReservations, table)

	log.Println("UPDATE: table: " +
		strconv.Itoa(table) + " | num_reserved: " +
		strconv.Itoa(totalReservations))
	defer db.Close()
}

// updateTableOccupation updates tables table when guest arrives and leaves
func updateTableOccupation(table int, totalOccupation int) {
	db := dbConn()
	insertForm, err := db.Prepare("UPDATE tables SET num_occupied=? WHERE table_id=?")
	if err != nil {
		panic(err.Error())
	}
	insertForm.Exec(totalOccupation, table)

	log.Println("UPDATE: table: " +
		strconv.Itoa(table) + " | num_occupied: " +
		strconv.Itoa(totalOccupation))
	defer db.Close()
}

// getTableReservationInfo retrieves table information of a guest before he leaves
func getTableReservationInfo(table_id int) (bool, int, int) {
	db := dbConn()
	var num_reserved, num_occupied = -1, -1
	selectDB, err := db.Query("SELECT num_reserved, num_occupied FROM tables WHERE table_id = ?", table_id)
	if err != nil {
		panic(err.Error())
	}
	for selectDB.Next() {
		err = selectDB.Scan(&num_reserved, &num_occupied)
		if err != nil {
			panic(err.Error())
		}
	}
	if num_reserved == -1 {
		return false, 0, 0
	}
	defer db.Close()
	return true, num_reserved, num_occupied
}

// ReturnGuestlist is a handler that returns a guest list as a response
func ReturnGuestlist(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Endpoint hit: returnGuestlist")
	db := dbConn()
	selectDB, err := db.Query("SELECT name, accompanying_guests, table_id FROM guests ORDER BY name")
	if err != nil {
		panic(err.Error())
	}
	guest := Guest{}
	guest_list := GuestList{}
	for selectDB.Next() {
		var accompanying_guests, table_id int
		var name string

		err = selectDB.Scan(
			&name,
			&accompanying_guests,
			&table_id,
		)
		guest.Name = name
		guest.AccompaniyingGuests = accompanying_guests
		guest.Table = table_id

		if err != nil {
			panic(err.Error())
		}
		guest_list.Guests = append(guest_list.Guests, guest)
	}

	defer db.Close()
	tmpl.ExecuteTemplate(w, "Guestlist", guest_list.Guests)
}

// GetHomePage is a handler that returns Index page
func GetHomePage(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Endpoint Hit: homepage")
	tmpl.ExecuteTemplate(w, "Index", nil)
}

// handleRequests is function that aggregates all request handlers together
func handleRequests() {
	myRouter := mux.NewRouter().StrictSlash(true)
	myRouter.HandleFunc("/", GetHomePage)
	myRouter.HandleFunc("/guest_list", ReturnGuestlist).Methods("GET")
	myRouter.HandleFunc("/guest_list/{name}", AddGuestToGuestlist).Methods("POST")
	myRouter.HandleFunc("/guests", GetArrivedGuests).Methods("GET")
	myRouter.HandleFunc("/guests/{name}", RegisterArrivedGuest).Methods("PUT")
	myRouter.HandleFunc("/guests/{name}", RegisterLeftGuest).Methods("DELETE")
	myRouter.HandleFunc("/seats_empty", CountSeatsEmpty).Methods("GET")

	log.Fatal(http.ListenAndServe(":10000", myRouter))
}

func main() {
	fmt.Println("Started serving guests")
	handleRequests()
}
