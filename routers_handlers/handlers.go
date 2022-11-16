package routers_handlers

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

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
	Period    string `json:"period"`
}

func makeTransaction(context *gin.Context, dataBase *sql.DB, sqlQueryUserBalance string, sqlQueryTransaction string) {
	tx, errBeginTx := dataBase.BeginTx(context, nil)
	if errBeginTx != nil {
		log.Print("errTx: ", errBeginTx)
		context.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "Internal server error"})
		return
	}
	defer tx.Rollback()

	_, errUpdateUserBalance := tx.ExecContext(context, sqlQueryUserBalance)
	if errUpdateUserBalance != nil {
		log.Print("errUpdateUserBalance: ", errUpdateUserBalance)
		context.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "Internal server error"})
		return
	}

	_, errInsertTransaction := tx.ExecContext(context, sqlQueryTransaction)
	if errInsertTransaction != nil {
		log.Print("errInsertTransaction: ", errInsertTransaction)
		context.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "Internal server error"})
		return
	}

	if errTxCommit := tx.Commit(); errTxCommit != nil {
		log.Print("errTxCommit: ", errTxCommit)
		context.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "Internal server error"})
		return
	}
}

func TopUpUserBalance(context *gin.Context, dataBase *sql.DB) {
	var userBalance UserBalance
	var transaction Transaction
	userBalance.Balance = 0

	if errBindJSONTopUp := context.BindJSON(&transaction); errBindJSONTopUp != nil {
		log.Print("errBindJSONTopUp: ", errBindJSONTopUp)
		context.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "Internal server error"})
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
	sqlQueryTransaction := fmt.Sprintf("insert into transactions (type, user_id, amount) "+
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

	if errBindJSONReserve := context.BindJSON(&transaction); errBindJSONReserve != nil {
		log.Print("errBindJSONReserve: ", errBindJSONReserve)
		context.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "Internal server error"})
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
	sqlQueryTransaction := fmt.Sprintf("insert into transactions (type, user_id, order_id, service_id, amount)"+
		"values ('%s', %d, %d, %d, %d)", transaction.Type, transaction.UserID, transaction.OrderID, transaction.ServiceID,
		transaction.Amount)

	makeTransaction(context, dataBase, sqlQueryUserBalance, sqlQueryTransaction)

	dataBase.Close()
	context.JSON(http.StatusOK, gin.H{"message": "Reserve successful"})
}

func ReserveWriteOff(context *gin.Context, dataBase *sql.DB) {
	var transaction Transaction
	var userBalance UserBalance

	if errBindJSONWriteOff := context.BindJSON(&transaction); errBindJSONWriteOff != nil {
		log.Print("errBindJSONWriteOff: ", errBindJSONWriteOff)
		context.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "Internal server error"})
		return
	}

	rowsTransaction, errGetTransaction := dataBase.Query("select transactions.type, transactions.amount from transactions "+
		"where transactions.user_id = $1 and transactions.order_id = $2 "+
		"and transactions.service_id = $3",
		transaction.UserID, transaction.OrderID, transaction.ServiceID)
	if errGetTransaction != nil {
		context.JSON(http.StatusNotFound, gin.H{"message": "Reserve transaction not found"})
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
	sqlQueryTransaction := fmt.Sprintf("insert into transactions (type, user_id, order_id, service_id, amount)"+
		"values ('%s', %d, %d, %d, %d)", transaction.Type, transaction.UserID, transaction.OrderID, transaction.ServiceID,
		transaction.Amount)

	makeTransaction(context, dataBase, sqlQueryUserBalance, sqlQueryTransaction)

	dataBase.Close()
	context.JSON(http.StatusOK, gin.H{"message": "Write-off successful"})
}

func RevenueReport(context *gin.Context, dataBase *sql.DB) {
	var transaction Transaction
	type RevenueReport struct {
		ServiceID int `json:"service_id"`
		Amount    int `json:"amount"`
	}
	var revenueReport []RevenueReport

	if errBindJSONPeriod := context.BindJSON(&transaction); errBindJSONPeriod != nil {
		log.Print("errBindJSONPeriod: ", errBindJSONPeriod)
		context.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "Internal server error"})
		return
	}

	yearStart, _ := strconv.Atoi(transaction.Period[0:4])
	monthStart, _ := strconv.Atoi(transaction.Period[5:7])
	dateStart := time.Date(yearStart, time.Month(monthStart), 1, 0, 0, 0, 0, time.UTC)
	dateEnd := dateStart.AddDate(0, 1, 0)
	periodStart := "'" + transaction.Period[0:4] + "-" + transaction.Period[5:7] + "-01" + "'"
	periodEnd := "'" + dateEnd.String()[0:7] + "-01" + "'"

	rowsRevenueReportPeriod, errGetReport := dataBase.Query("select transactions.service_id, sum(transactions.amount) as amount "+
		"from transactions where transactions.timestamp < $1 and transactions.timestamp > $2 and "+
		"transactions.type = 'write-off' group by transactions.service_id", periodEnd, periodStart)

	if errGetReport != nil {
		log.Print("errGetReport: ", errGetReport)
		context.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "Internal server error"})
		return
	}
	dataBase.Close()
	defer rowsRevenueReportPeriod.Close()

	for rowsRevenueReportPeriod.Next() {
		rowsRevenueReportPeriod.Scan(&transaction.ServiceID, &transaction.Amount)
		revenueReport = append(revenueReport, RevenueReport{transaction.ServiceID, transaction.Amount})
	}
	rowsRevenueReportPeriod.Close()

	context.JSON(http.StatusOK, revenueReport)
}
