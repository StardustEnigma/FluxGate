package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/StardustEnigma/FluxGate/routes"

)

func main(){

	fmt.Println("Starting server at port 80")
	router := routes.Routes()
	if err := http.ListenAndServe(":8080",router);err !=nil{
		log.Fatal(err)
	}
}
