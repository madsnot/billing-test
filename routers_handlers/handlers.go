package routers_handlers

import (
	"database/sql"
	"fmt"
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

func makeTransaction(context *gin.Context, dataBase *sql.DB, sqlQueryUserBalance string, sqlQueryTransaction string) {
	tx, errBeginTx := dataBase.BeginTx(context, nil)
	if errBeginTx != nil {
		log.Fatal("errTx: ", errBeginTx)
		return
	}
	defer tx.Rollback()

	_, errUpdateUserBalance := tx.ExecContext(context, sqlQueryUserBalance)
	if errUpdateUserBalance != nil {
		log.Fatal("errUpdateUserBalance: ", errUpdateUserBalance)
		return
	}

	_, errInsertTransaction := tx.ExecContext(context, sqlQueryTransaction)
	if errInsertTransaction != nil {
		log.Fatal("errInsertTransaction: ", errInsertTransaction)
		return
	}

	if errTxCommit := tx.Commit(); errTxCommit != nil {
		log.Fatal("errTxCommit: ", errTxCommit)
		return
	}
}

func TopUpUserBalance(context *gin.Context, dataBase *sql.DB) {
	var userBalance UserBalance
	var transaction Transaction
	userBalance.Balance = 0

	if errBindJSON := context.BindJSON(&transaction); errBindJSON != nil {
		log.Fatal("errBindJSON: ", errBindJSON)
		return
	}

	rowUserBalance := dataBase.QueryRow("select user_balance.balance from user_balance "+
		"where user_balance.user_id = $1", transaction.UserID)

	sqlQueryUserBalance := fmt.Sprintf("insert into user_balance (user_id, balance) values (%d, %d)",
		transaction.UserID, transaction.Amount)
	if errRowScan := rowUserBalance.Scan(&userBalance.Balance); errRowScan == nil {
		userBalance.Balance += transaction.Amount
		sqlQueryUserBalance = fmt.Sprintf("update user_balance set balance = %d where user_id = %d",
			userBalance.Balance, transaction.UserID)
	}

	transaction.Type = "top_up"
	sqlQueryTransaction := fmt.Sprintf("insert into transaction (type, user_id, amount) "+
		"values ('%s', %d, %d)", transaction.Type, transaction.UserID, transaction.Amount)

	makeTransaction(context, dataBase, sqlQueryUserBalance, sqlQueryTransaction)

	dataBase.Close()
	context.JSON(http.StatusOK, "Top up successful")
}

func GetUserBalance(context *gin.Context, dataBase *sql.DB) {
	var userBalance UserBalance

	userBalance.UserID, _ = strconv.Atoi(context.Param("userId"))

	rowUserBalance := dataBase.QueryRow("select user_balance.balance from user_balance "+
		"where user_balance.user_id = $1", userBalance.UserID)

	if errRowScan := rowUserBalance.Scan(&userBalance.Balance); errRowScan != nil {
		context.JSON(http.StatusNotFound, gin.H{"message": "User not found"})
		return
	}

	dataBase.Close()
	context.JSON(http.StatusOK, userBalance.Balance)
}

func ReserveAmountForPayment(context *gin.Context, dataBase *sql.DB) {
	var userBalance UserBalance
	var transaction Transaction

	if errBindJSON := context.BindJSON(&transaction); errBindJSON != nil {
		log.Fatal("errBindJSON: ", errBindJSON)
		return
	}

	rowUserBalance := dataBase.QueryRow("select user_balance.balance from user_balance "+
		"where user_balance.user_id = $1", transaction.UserID)

	if errRowScan := rowUserBalance.Scan(&userBalance.Balance); errRowScan != nil {
		context.JSON(http.StatusNotFound, gin.H{"message": "User not found"})
		return
	}

	if userBalance.Balance < transaction.Amount {
		context.JSON(http.StatusBadRequest, gin.H{"message": "Account has insufficient funds"})
		return
	}
	userBalance.Balance -= transaction.Amount
	transaction.Type = "reserve"

	sqlQueryUserBalance := fmt.Sprintf("update user_balance set balance = %d, reserved_balance = %d where user_id = %d",
		userBalance.Balance, transaction.Amount, transaction.UserID)
	sqlQueryTransaction := fmt.Sprintf("insert into transaction (type, user_id, order_id, service_id, amount)"+
		"values ('%s', %d, %d, %d, %d)", transaction.Type, transaction.UserID, transaction.OrderID, transaction.ServiceID,
		transaction.Amount)

	makeTransaction(context, dataBase, sqlQueryUserBalance, sqlQueryTransaction)

	dataBase.Close()
	context.JSON(http.StatusOK, gin.H{"message": "Reserve successful"})
}

func ReserveWriteOff(context *gin.Context, dataBase *sql.DB) {
	var transaction Transaction
	var userBalance UserBalance

	if errBindJSON := context.BindJSON(&transaction); errBindJSON != nil {
		log.Fatal("errBindJSON: ", errBindJSON)
		return
	}

	rowsTransaction, errGetTransaction := dataBase.Query("select transaction.type, transaction.amount from transaction "+
		"where transaction.user_id = $1 and transaction.order_id = $2 "+
		"and transaction.service_id = $3",
		transaction.UserID, transaction.OrderID, transaction.ServiceID)
	if errGetTransaction != nil {
		context.JSON(http.StatusNotFound, gin.H{"message": "Reserve amount transaction not found"})
		return
	}
	defer rowsTransaction.Close()

	for rowsTransaction.Next() {
		rowsTransaction.Scan(&transaction.Type, &userBalance.ReservedBalance)
		if transaction.Type == "write-off" {
			context.JSON(http.StatusBadRequest, gin.H{"message": "Write-off has already done"})
			rowsTransaction.Close()
			return
		}
	}

	if transaction.Type != "reserve" {
		context.JSON(http.StatusBadRequest, gin.H{"message": "The order is not found"})
		return
	}

	if userBalance.ReservedBalance != transaction.Amount {
		context.JSON(http.StatusBadRequest, gin.H{"message": "Error the order amount"})
		return
	}

	userBalance.ReservedBalance -= transaction.Amount
	transaction.Type = "write-off"
	sqlQueryUserBalance := fmt.Sprintf("update user_balance set reserved_balance = %d "+
		"where user_id = %d", userBalance.ReservedBalance, transaction.UserID)
	sqlQueryTransaction := fmt.Sprintf("insert into transaction (type, user_id, order_id, service_id, amount)"+
		"values ('%s', %d, %d, %d, %d)", transaction.Type, transaction.UserID, transaction.OrderID, transaction.ServiceID,
		transaction.Amount)

	makeTransaction(context, dataBase, sqlQueryUserBalance, sqlQueryTransaction)

	dataBase.Close()
	context.JSON(http.StatusOK, gin.H{"message": "Write-off successful"})
}
