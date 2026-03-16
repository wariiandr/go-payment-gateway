package payment

type PaymentStatus string

const (
	New        PaymentStatus = "new"
	Processing PaymentStatus = "processing"
	Completed  PaymentStatus = "completed"
	Failed     PaymentStatus = "failed"
	Canceled   PaymentStatus = "canceled"
)

func CanTransition(from PaymentStatus, to PaymentStatus) bool {
	switch from {
	case New:
		return to == Processing || to == Canceled
	case Processing:
		return to == Completed || to == Failed || to == Canceled
	default:
		return false
	}
}
