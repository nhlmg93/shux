package actor

import "shux-dev/internal/protocol"

// EventRef names the optional hub pointer used when publishing lifecycle events.
type EventRef = *Ref[protocol.Event]
