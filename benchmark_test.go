package main

import (
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"hasucalc/tui"
)

func BenchmarkCursorMovement(b *testing.B) {
	s := tcell.NewSimulationScreen("")
	s.Init()
	s.SetSize(80, 25)
	defer s.Fini()

	sh := createDemoSheet()
	app := tui.NewApp(s, sh, "DATA.WK3")

	start := time.Now()
	for i := 0; i < b.N; i++ {
		keyEv := tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		_ = keyEv
		// Move down then up
		for step := 0; step < 50; step++ {
			evDown := tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
			// simulate key
			app.ProcessEventForTest(evDown)
		}
		for step := 0; step < 50; step++ {
			evUp := tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
			app.ProcessEventForTest(evUp)
		}
	}
	elapsed := time.Since(start)
	b.Logf("Time for %d iterations (100 movements each): %v", b.N, elapsed)
}
