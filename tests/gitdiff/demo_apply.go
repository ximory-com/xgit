package main

// demo_apply.go：最小可用桩
type Patch struct {
	Ops []*FileOp
}

func ApplyOnce(logger DualLogger, repo string, p *Patch) {
	// no-op for smoke test
}
