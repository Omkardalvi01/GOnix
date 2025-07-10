package main

import (
	"fmt"
	"log"
	"net/http"
)

func test_response(w http.ResponseWriter , r *http.Request){
	resp := fmt.Sprintf("Hello from the backend for %v", r.URL)
	fmt.Fprint(w , resp)
}

func main(){
	http.HandleFunc("/user/v1", test_response)
	http.HandleFunc("/user/v2", test_response)
	http.HandleFunc("/user/v3", test_response)
	http.HandleFunc("/admin", test_response)

	log.Fatal(http.ListenAndServe(":4000", nil))
}
