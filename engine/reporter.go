package engine

// Reporter receives live updates during a run. Implementations may render to
// a TTY, push events to a GUI, persist to disk, or compose multiple of those.
// Methods should be non-blocking; slow reporters risk being dropped by the
// runner rather than back-pressuring it.
type Reporter interface {
	OnStart(Plan)
	OnTick(Snapshot) // emitted approximately every 250ms during the run
	OnFinish(*Summary)
}
