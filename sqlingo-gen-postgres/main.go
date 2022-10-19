package main

import (
	_ "github.com/lib/pq"
	"github.com/lqs/sqlingo/generator"
)

func main() {
	err := generator.Generate("postgres", "host=localhost port=5432 user=user password=pass dbname=db sslmode=disable")
	if err != nil {
		panic(err)
	}
}
