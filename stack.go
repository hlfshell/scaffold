package scaffold

/*
Stack is a simple ordered collection of service groups. Services passed
in the same WithServices call are created in parallel. Multiple
WithServices calls are created in the order they are applied.
*/
type Stack struct {
	name            string
	services        []Service
	serviceGroups   [][]Service
	createdGroups   [][]Service
	networkName     string
	sharedNetwork   bool
	networkCreated  bool
	runID           string
	namePrefix      string
	created         bool
	creating        bool
	cleaning        bool
	inheritedLabels map[string]string
	userLabels      map[string]string
}

// StackOption configures a stack at construction time.
type StackOption func(*Stack)

/*
NewStack builds a stack with the provided options. A stack can contain
any service harness that implements the Service interface.
*/
func NewStack(name string, options ...StackOption) *Stack {
	stack := &Stack{
		name:            name,
		services:        []Service{},
		serviceGroups:   [][]Service{},
		createdGroups:   [][]Service{},
		inheritedLabels: map[string]string{},
		userLabels:      map[string]string{},
	}

	for _, option := range options {
		option(stack)
	}

	return stack
}

func (s *Stack) Name() string {
	return s.name
}

/*
SetLabels merges labels inherited from a parent stack. Child services and
child stacks receive these labels before they are created.
*/
func (s *Stack) SetLabels(labels map[string]string) {
	s.inheritedLabels = mergeLabels(s.inheritedLabels, labels)
}

/*
SetNetwork pushes an inherited Docker network into this stack and its children.
A stack with an inherited network does not own that network. Children services
should adopt the network upon start.
*/
func (s *Stack) SetNetwork(name string) {
	s.sharedNetwork = false
	s.networkName = name

	for _, service := range s.services {
		attachable, ok := service.(NetworkAttachable)
		if ok {
			attachable.SetNetwork(name)
		}
	}
}

/*
SetNamePrefix receives a parent stack's prefix. Child services receive
that prefix plus this stack's name when the stack is created.
*/
func (s *Stack) SetNamePrefix(prefix string) {
	s.namePrefix = prefix
}

func (s *Stack) Services() []Service {
	services := make([]Service, len(s.services))
	copy(services, s.services)
	return services
}

/*
Service returns a service by name if it exists in the stack; nil and false if
not.
*/
func (s *Stack) Service(name string) (Service, bool) {
	for _, service := range s.services {
		if service.Name() == name {
			return service, true
		}
	}

	return nil, false
}
