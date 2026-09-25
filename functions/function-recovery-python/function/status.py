"""Observer-based Operation status reporting."""

from dataclasses import dataclass
from typing import Protocol

from crossplane.function import response
from crossplane.function.proto.v1 import run_function_pb2 as fnv1


@dataclass(frozen=True)
class OperationStatusEvent:
    """A status update emitted by one recovery operation."""

    operation: str
    status: str
    message: str


class OperationStatusObserver(Protocol):
    """Receive Operation status updates."""

    def update(self, event: OperationStatusEvent) -> None: ...


class OperationStatusSubject:
    """Publish status updates to registered observers."""

    def __init__(self) -> None:
        self._observers: list[OperationStatusObserver] = []

    def attach(self, observer: OperationStatusObserver) -> None:
        self._observers.append(observer)

    def detach(self, observer: OperationStatusObserver) -> None:
        self._observers.remove(observer)

    def notify(self, event: OperationStatusEvent) -> None:
        for observer in self._observers:
            observer.update(event)


class FunctionResponseStatusObserver:
    """Write operation status events to the Crossplane function response."""

    def __init__(self, rsp: fnv1.RunFunctionResponse) -> None:
        self._response = rsp

    def update(self, event: OperationStatusEvent) -> None:
        response.set_output(
            self._response,
            {
                "operation": event.operation,
                "status": event.status,
                "message": event.message,
            },
        )