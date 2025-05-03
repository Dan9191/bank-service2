package models

// IncomeExpenseStats represents monthly income and expense statistics
type IncomeExpenseStats struct {
	Income     float64 `json:"income"`
	Expense    float64 `json:"expense"`
	NetBalance float64 `json:"net_balance"`
}

// CreditBurden represents credit burden analytics
type CreditBurden struct {
	MonthlyPayments float64 `json:"monthly_payments"`
	TotalBalance    float64 `json:"total_balance"`
	BurdenRatio     float64 `json:"burden_ratio"` // MonthlyPayments / TotalBalance
}

// BalanceForecast represents balance forecast for N days
type BalanceForecast struct {
	InitialBalance float64        `json:"initial_balance"`
	ForecastedDays int            `json:"forecasted_days"`
	DailyForecast  []DailyBalance `json:"daily_forecast"`
}

// DailyBalance represents balance for a specific day
type DailyBalance struct {
	Date    string  `json:"date"` // Format: YYYY-MM-DD
	Balance float64 `json:"balance"`
}
