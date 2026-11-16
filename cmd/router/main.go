package main

import (
	"log"
	"net/http"
	"os"
	"reddit-lab/internal/router"
	"strconv"
)

func main() {
	percent, _ := strconv.Atoi(os.Getenv("GO_PERCENT"))
	rt, err := router.New(os.Getenv("LEGACY_URL"), os.Getenv("GO_URL"), percent)
	if err != nil {
		log.Fatalf("router: %v", err)
	}
	log.Printf("router listening on :8080 (go=%d%%)", percent)
	log.Fatal(http.ListenAndServe(":8080", rt.Handler()))
	
}
