package main

import (
	"database/sql"
	"example/billing-test/config"
	"example/billing-test/routers_handlers"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func ValidOpenDataBase(function func(*gin.Context, *sql.DB)) gin.HandlerFunc {

	return func(context *gin.Context) {
		conf := config.New()
		dataBaseUser := conf.DataBase.User
		dataBasePass := conf.DataBase.Pass
		dataBaseName := conf.DataBase.Name
		dataBaseSSLMode := conf.DataBase.SSLMode
		dataBaseDriver := conf.DataBase.Driver
		connStr := "user=" + dataBaseUser + " password=" + dataBasePass + " dbname=" +
			dataBaseName + " sslmode=" + dataBaseSSLMode
		dataBase, errOpenDB := sql.Open(dataBaseDriver, connStr)
		if errOpenDB != nil {
			log.Print("errOpenDB: ", errOpenDB)
			context.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "Internal server error"})
			return
		}
		defer dataBase.Close()

		function(context, dataBase)
	}
}

func init() {
	if errLoadEnv := godotenv.Load(); errLoadEnv != nil {
		log.Print("File .env not found")
	}
}

func main() {
	router := gin.Default()

	router.POST("/balance/topUp", ValidOpenDataBase(routers_handlers.TopUpUserBalance))
	router.GET("/balance/:userId", ValidOpenDataBase(routers_handlers.GetUserBalance))
	router.POST("/payment/reserve", ValidOpenDataBase(routers_handlers.ReserveAmountForPayment))
	router.POST("/payment", ValidOpenDataBase(routers_handlers.ReserveWriteOff))
	router.POST("/report", ValidOpenDataBase(routers_handlers.RevenueReport))

	router.Run(":8080")
}
