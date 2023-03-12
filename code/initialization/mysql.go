package initialization

import (
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

var db *sqlx.DB

func InitMysqlClient(config Config) {
	var err error
	db, err = sqlx.Open("mysql", config.Mysql)
	if err != nil {
		panic(err)
	}
}

func GetMysqlClient() *sqlx.DB {
	return db
}
