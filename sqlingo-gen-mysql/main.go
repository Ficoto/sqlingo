package main

import (
	"github.com/Ficoto/sqlingo/generator"
	_ "github.com/go-sql-driver/mysql"
)

func main() {
	err := generator.Generate("mysql", "username:password@tcp(hostname:3306)/database")
	if err != nil {
		panic(err)
	}
}
