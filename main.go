package main

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

type backend struct{}

func (b backend) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Some Backend opertaion rawr")
}

func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("url : %v time : %v method: %v\n", r.URL, time.Now(), r.Method)
		next.ServeHTTP(w, r)
	})
}

func main() {
	fmt.Println("Listening at 5000/bleh")
	http.Handle("/bleh", logging(new(backend)))

	log.Fatal(http.ListenAndServe(":5000", nil))

}
