package sentry

import (
	raven "github.com/getsentry/raven-go"
)

// Capture reports an error message to sentry along with a stacktrace.
//
// skipFrames can be used to indicate how many stack frames should be skipped to
// reach the frame where the error actually happened instead of always pointing
// to a generic "logError()".
//
// tags are additional tags to add the sentry event.
//
// This call is synchronous in that it will not return until the message has
// been sent to sentry.
func Capture(message string, skipFrames int, tags map[string]string) {
	// We want to at least skip the Capture() function itself.
	if skipFrames < 1 {
		skipFrames = 1
	}

	// Capture the error with stacktrace and wait.
	stack := raven.NewStacktrace(skipFrames, 3, nil)
	packet := raven.NewPacket(message, stack)
	eventID, ch := raven.Capture(packet, tags)
	if eventID != "" {
		<-ch
	}
}
