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
	var transaction Transaction
	userBalance.Balance = 0

	if errBindJSON := context.BindJSON(&transaction); errBindJSON != nil {
		log.Fatal("errBindJSON: ", errBindJSON)
		return
	}

	rowUserBalance := dataBase.QueryRow("select user_balance.balance from user_balance "+
		"where user_balance.user_id = $1", transaction.UserID)

	if errRowScan := rowUserBalance.Scan(&userBalance.Balance); errRowScan != nil {
		_, errInsert := dataBase.Exec("insert into user_balance (user_id, balance) values ($1, $2)",
			transaction.UserID, transaction.Amount)
		if errInsert != nil {
			log.Fatal("errInsert: ", errInsert)
			return
		}
	} else {
		userBalance.Balance += transaction.Amount
		_, errUpdate := dataBase.Exec("update user_balance set balance = $1 where user_id = $2",
			userBalance.Balance, transaction.UserID)
		if errUpdate != nil {
			log.Fatal("errUpdate: ", errUpdate)
			return
		}
	}

	transaction.Type = "top_up"
	_, errMakeTopUpTransaction := dataBase.Exec("insert into transaction (type, user_id, amount)"+
		"values ($1, $2, $3)", transaction.Type, transaction.UserID, transaction.Amount)
	if errMakeTopUpTransaction != nil {
		log.Fatal("errMakeTopUpTransaction: ", errMakeTopUpTransaction)
		return
	}

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

	_, errReserveAmount := dataBase.Exec("update user_balance set balance = $1, reserved_balance = $2 where user_id = $3",
		userBalance.Balance, transaction.Amount, transaction.UserID)
	if errReserveAmount != nil {
		log.Fatal("errReserveAmount: ", errReserveAmount)
		return
	}

	transaction.Type = "reserve"
	_, errMakeReserveTransaction := dataBase.Exec("insert into transaction (type, user_id, order_id, service_id, amount)"+
		"values ($1, $2, $3, $4, $5)", transaction.Type, transaction.UserID, transaction.OrderID, transaction.ServiceID,
		transaction.Amount)
	if errMakeReserveTransaction != nil {
		log.Fatal("errMakeReserveTransaction: ", errMakeReserveTransaction)
		return
	}
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

	if userBalance.ReservedBalance < transaction.Amount {
		context.JSON(http.StatusBadRequest, gin.H{"message": "Error the order amount"})
		return
	}

	userBalance.ReservedBalance -= transaction.Amount
	transaction.Type = "write-off"
	tx, errTx := dataBase.BeginTx(context, nil)
	if errTx != nil {
		log.Fatal("errTx: ", errTx)
		return
	}
	defer tx.Rollback()

	_, errWriteOffReserveAmount := tx.ExecContext(context, "update user_balance set reserved_balance = $1 "+
		"where user_id = $2", userBalance.ReservedBalance, transaction.UserID)
	if errWriteOffReserveAmount != nil {
		log.Fatal("errWriteOffReserveAmount: ", errWriteOffReserveAmount)
		return
	}

	_, errMakeWriteOffTransaction := tx.ExecContext(context, "insert into transaction (type, user_id, order_id, service_id, amount)"+
		"values ($1, $2, $3, $4, $5)", transaction.Type, transaction.UserID, transaction.OrderID, transaction.ServiceID,
		transaction.Amount)
	if errMakeWriteOffTransaction != nil {
		log.Fatal("errMakeWriteOffTransaction: ", errMakeWriteOffTransaction)
		return
	}

	if errTxCommit := tx.Commit(); errTxCommit != nil {
		log.Fatal("errTxCommit: ", errTxCommit)
		return
	}

	dataBase.Close()
	context.JSON(http.StatusOK, gin.H{"message": "Write-off successful"})
}
