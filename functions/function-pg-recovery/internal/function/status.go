package function

import (
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/response"
	"github.com/melak-cmd/crossplane-labs/functions/function-pg-recovery/internal/model"
)

type OperationStatusEvent struct {
	Operation model.Operation
	Status    string
	Message   string
}

type OperationStatusObserver interface {
	Update(OperationStatusEvent)
}

type OperationStatusSubject struct {
	observers []OperationStatusObserver
}

func newOperationStatusSubject(rsp *fnv1.RunFunctionResponse) *OperationStatusSubject {
	subject := &OperationStatusSubject{}
	subject.Attach(responseStatusObserver{rsp: rsp})
	return subject
}

func (s *OperationStatusSubject) Attach(observer OperationStatusObserver) {
	s.observers = append(s.observers, observer)
}

func (s *OperationStatusSubject) Notify(event OperationStatusEvent) {
	for _, observer := range s.observers {
		observer.Update(event)
	}
}

type responseStatusObserver struct {
	rsp *fnv1.RunFunctionResponse
}

func (o responseStatusObserver) Update(event OperationStatusEvent) {
	if err := response.SetOutput(o.rsp, map[string]string{
		"operation": string(event.Operation),
		"status":    event.Status,
		"message":   event.Message,
	}); err != nil {
		Fatal(o.rsp, err, "cannot set operation status output")
	}
}
