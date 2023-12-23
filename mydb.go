package main

import (
	"fmt"
	"gorm.io/driver/mysql"
	//"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"time"
)

var db *gorm.DB

func GetDb() *gorm.DB {
	return db
}

func initDB(host string, databaseName string, dbuser string, dbpassword string, dbport int) {
	var err error
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8&parseTime=True&loc=Local&timeout=5s", dbuser, dbpassword, host, dbport, databaseName)
	db, err = gorm.Open(mysql.New(mysql.Config{
		DSN:                       dsn,
		DefaultStringSize:         256,
		DisableDatetimePrecision:  true,
		DontSupportRenameIndex:    true,
		DontSupportRenameColumn:   true,
		SkipInitializeWithVersion: false}),
		&gorm.Config{
			PrepareStmt:            true,
			SkipDefaultTransaction: true,
		},
	)
	if err != nil {
		panic("Failed to connect to databaseName:" + err.Error())
		return
	}
	sqlDB, err := db.DB()
	sqlDB.SetMaxIdleConns(100)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(5 * time.Second)
	err = initDbTable()
	if err != nil {
		panic(err)
		return
	}
}

func initDbTable() error {
	db := GetDb()
	fmt.Println(db)
	return nil
}
