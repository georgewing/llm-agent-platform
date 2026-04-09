package kernel

import "time"

type DomainEvent interface {
	EventName() string
	OccurredAt() time.Time
}

type EventPublisher interface {
	Publish(event DomainEvent) error
}
