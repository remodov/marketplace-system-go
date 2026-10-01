package system

import (
	"time"

	"github.com/google/uuid"
)

type Clock struct{}

func (Clock) Now() time.Time { return time.Now().UTC() }

type IDGenerator struct{}

func (IDGenerator) NewID() uuid.UUID { return uuid.New() }
