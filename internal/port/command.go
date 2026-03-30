package port

type CommandResult string

const (
	CommandResultCompleted CommandResult = "completed"
	CommandResultFailed    CommandResult = "failed"
)
