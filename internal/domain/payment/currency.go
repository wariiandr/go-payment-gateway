package payment

type Currency string

const (
	RUB Currency = "RUB"
	USD Currency = "USD"
)

func (c Currency) IsValid() bool {
	switch c {
	case RUB, USD:
		return true
	default:
		return false
	}
}
