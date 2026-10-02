package httpapi_test

import "github.com/shopspring/decimal"

func decimalOf(s string) decimal.Decimal { return decimal.RequireFromString(s) }
