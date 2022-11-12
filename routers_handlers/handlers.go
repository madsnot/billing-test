package routers_handlers

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

type UserBalance struct {
	ID              int `json:"id"`
	UserID          int `json:"userId"`
	Balance         int `json:"balance"`
	ReservedBalance int `json:"reserve"`
}

type Transaction struct {
	ID        int    `json:"id"`
	Type      string `json:"type"`
	UserID    int    `json:"userId"`
	OrderID   int    `json:"orderId"`
	ServiceID int    `json:"serviceId"`
	Amount    int    `json:"amount"`
}

func TopUpUserBalance(context *gin.Context, dataBase *sql.DB) {
	var userBalance UserBalance
	balance := 0

	if errBindJSON := context.BindJSON(&userBalance); errBindJSON != nil {
		log.Fatal("errBindJSON: ", errBindJSON)
		return
	}

	rowsUserBalance := dataBase.QueryRow("select user_balance.balance from user_balance "+
		"where user_balance.user_id = $1", userBalance.UserID)

	if errRowScan := rowsUserBalance.Scan(&balance); errRowScan != nil {
		_, errInsert := dataBase.Exec("insert into user_balance (user_id, balance) values ($1, $2)",
			userBalance.UserID, userBalance.Balance)
		if errInsert != nil {
			log.Fatal("errInsert: ", errInsert)
			return
		}
	} else {
		userBalance.Balance += balance
		_, errUpdate := dataBase.Exec("update user_balance set balance = $1 where user_id = $2",
			userBalance.Balance, userBalance.UserID)
		if errUpdate != nil {
			log.Fatal("errUpdate: ", errUpdate)
			return
		}
	}
	dataBase.Close()
	context.JSON(http.StatusOK, "Balance has been replenished")
}

func GetUserBalance(context *gin.Context, dataBase *sql.DB) {
	var userBalance UserBalance

	userBalance.UserID, _ = strconv.Atoi(context.Param("userId"))

	rowsUserBalance := dataBase.QueryRow("select user_balance.balance from user_balance "+
		"where user_balance.user_id = $1", userBalance.UserID)

	if errRowScan := rowsUserBalance.Scan(&userBalance.Balance); errRowScan != nil {
		context.JSON(http.StatusNotFound, gin.H{"message": "User not found"})
		return
	}
	dataBase.Close()
	context.JSON(http.StatusOK, userBalance.Balance)
}

func ReserveAmountForPayment(context *gin.Context, dataBase *sql.DB) {

	//if UserBalance.Balance, err = (получение из базы в UserBalance по userID); UserBalance.Balance == 0 || err != nil
	// {context.JSON(http.StatusBadRequest, gin.H{"message:", "Account has insufficient funds"}) return}

}

func ReserveWriteOff(context *gin.Context, dataBase *sql.DB) {

}
