package main

import (
	"encoding/json"
	"log"
	"net/http"
)

func main() {
	log.Print("Listening 8000")
	http.HandleFunc("/", blogHandler)
	log.Fatal(http.ListenAndServe(":8000", nil))
}

func blogHandler(w http.ResponseWriter, r *http.Request) {
	db := GetDB()
	print("got db")

	rows, err := db.Query("SELECT title FROM blog")
	if err != nil {
		w.WriteHeader(500)
		return
	}
	var titles []string
	for rows.Next() {
		var title string
		err = rows.Scan(&title)
		titles = append(titles, title)
	}
	json.NewEncoder(w).Encode(titles)
}
