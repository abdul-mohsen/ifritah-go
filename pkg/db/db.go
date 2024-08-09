package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/go-sql-driver/mysql"
)

func Connect() *sql.DB {
  // Capture connection properties.
  cfg := mysql.Config{
    User:   os.Getenv("USER"),
    Passwd: os.Getenv("PASSWORD"),
    Net:    "tcp",
    Addr:   os.Getenv("HOST"),
    DBName: os.Getenv("DBNAME"),
    AllowNativePasswords: true,
  }
  log.Fatal(cfg)
  // Get a database handle.
  var db *sql.DB
  var err error
  db, err = sql.Open("mysql", cfg.FormatDSN())
  if err != nil {
    log.Fatal(err)
  }

  pingErr := db.Ping()
  if pingErr != nil {
    
    log.Fatal(os.Getenv("USER"),)
    
    log.Fatal(os.Getenv("PASSWORD"))
    log.Fatal(os.Getenv("HOST"))
    log.Fatal(os.Getenv("DBNAME"))
    
    log.Fatal(pingErr)
  }
  fmt.Println("Connected!")
  return db

}

func CloseConnection(db *sql.DB) {
  defer db.Close()
}

