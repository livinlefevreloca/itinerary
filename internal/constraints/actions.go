package constraints

// NoOpAction does nothing - used for testing constraint checker integration
// Full action implementations belong in the actions module
type NoOpAction struct {
	name string
}

func NewNoOpAction(name string) *NoOpAction {
	return &NoOpAction{name: name}
}

func (n *NoOpAction) Execute(ctx *ExecutionContext) error {
	return nil
}

func (n *NoOpAction) Name() string {
	return n.name
}
