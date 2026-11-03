package otel

// allowSetupAgain lets each test call Setup.
func allowSetupAgain() { setupCalled.Store(false) }
