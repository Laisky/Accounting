package imports

import (
	"regexp"
	"strings"
)

var transferAccountAmountPattern = regexp.MustCompile(`^(.+?)[：:]\s*([+-]?[0-9][0-9,]*(?:\.[0-9]+)?)$`)

// normalizeWacaiType receives a source type and returns the internal preview type.
func normalizeWacaiType(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "expense", "支出":
		return "expense"
	case "income", "收入":
		return "income"
	case "transfer", "转账":
		return "transfer"
	case "borrow", "借入":
		return "borrow"
	case "lend", "loan", "借出", "借贷":
		return "lend"
	case "repayment", "还款":
		return "repayment"
	case "refund", "退款":
		return "refund"
	case "reimbursement", "报销":
		return "reimbursement"
	default:
		return strings.ToLower(strings.TrimSpace(raw))
	}
}

// normalizeWacaiCurrency receives a source currency name and returns an ISO currency code.
func normalizeWacaiCurrency(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "":
		return ""
	case "cny", "rmb", "人民币", "中国元":
		return "CNY"
	case "usd", "美元", "美金":
		return "USD"
	case "cad", "加拿大元", "加元":
		return "CAD"
	case "eur", "欧元":
		return "EUR"
	default:
		return strings.ToUpper(strings.TrimSpace(raw))
	}
}

// parseWacaiAccounts receives a source account cell and returns source and destination account names.
func parseWacaiAccounts(raw string, rawType string) (string, string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ""
	}
	if normalizeWacaiType(rawType) != "transfer" {
		return raw, ""
	}

	var source string
	var destination string
	for _, part := range splitWacaiAccountParts(raw) {
		name, amount := parseWacaiAccountAmount(part)
		if name == "" {
			continue
		}
		switch {
		case strings.HasPrefix(amount, "-") && source == "":
			source = name
		case strings.HasPrefix(amount, "+") && destination == "":
			destination = name
		case source == "":
			source = name
		case destination == "":
			destination = name
		}
	}
	if source == "" && destination == "" {
		return raw, ""
	}

	return source, destination
}

// splitWacaiAccountParts receives a compound account cell and returns individual account segments.
func splitWacaiAccountParts(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == '，' || r == ',' || r == ';' || r == '|'
	})
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			values = append(values, part)
		}
	}

	return values
}

// parseWacaiAccountAmount receives one account segment and returns its account name and signed amount.
func parseWacaiAccountAmount(raw string) (string, string) {
	matches := transferAccountAmountPattern.FindStringSubmatch(strings.TrimSpace(raw))
	if len(matches) != 3 {
		return strings.TrimSpace(raw), ""
	}

	return strings.TrimSpace(matches[1]), strings.TrimSpace(matches[2])
}
