package tui

import (
	"fmt"
	"sync"
	stdtime "time"

	galatui "github.com/martianoff/gala-tui"
	server "github.com/martianoff/gala-server"
	collimm "martianoff/gala/collection_immutable"
	concurrent "martianoff/gala/concurrent"
	std "martianoff/gala/std"
	timeutils "martianoff/gala/time_utils"
)

// =============================================================================
// Filter — captures request events into the bus.
// =============================================================================

// TUIFilter returns a server.Filter that records each request's method, path,
// status, and latency to the given Bus for live dashboard display.
//
// Implemented in Go because cross-project sealed-case construction in GALA hits
// the unfixed lowering-path side of BUG-10 (see gala_tui/TRANSPILER_BUGS.md).
func TUIFilter(b *Bus) server.Filter {
	return func(req server.Request, next server.Handler) concurrent.Future[server.Response] {
		start := stdtime.Now()
		fut := next(req)
		return concurrent.Future_Map[server.Response, server.Response](fut, func(resp server.Response) server.Response {
			elapsedMs := stdtime.Since(start).Milliseconds()
			b.Push(NewEvent(req.Method().Name(), req.Path(), resp.Code(), elapsedMs))
			return resp
		})
	}
}

// =============================================================================
// Dashboard model
// =============================================================================

// dashboardModel is the Elm-style model for the live request dashboard.
type dashboardModel struct {
	bus        *Bus
	rps        []int      // last 60 seconds, samples-per-second
	perSecond  int        // current second's accumulator
	lastSecond int64      // unix seconds for the current bucket
	logs       galatui.LogPanel
	total      int
	startedAt  stdtime.Time
}

// dashboardMsg is the union of TUI events the dashboard reacts to.
type dashboardMsg struct {
	tick bool
	key  galatui.KeyEvent
}

func tickMsg() dashboardMsg              { return dashboardMsg{tick: true} }
func keyMsg(k galatui.KeyEvent) dashboardMsg { return dashboardMsg{key: k} }

func newDashboardModel(b *Bus) dashboardModel {
	return dashboardModel{
		bus:        b,
		rps:        make([]int, 60),
		perSecond:  0,
		lastSecond: stdtime.Now().Unix(),
		logs:       galatui.NewLogPanel(100).Info("dashboard ready — waiting for traffic"),
		total:      0,
		startedAt:  stdtime.Now(),
	}
}

// =============================================================================
// Update
// =============================================================================

func dashboardUpdate(m dashboardModel, msg dashboardMsg) std.Tuple[dashboardModel, galatui.Cmd[dashboardMsg]] {
	if !msg.tick {
		if galatui.KeyMatches(msg.key, "q") || galatui.KeyMatches(msg.key, "ctrl+c") {
			return tup(m, galatui.QuitCmd[dashboardMsg]{}.Apply())
		}
		return tup(m, galatui.NoCmd[dashboardMsg]{}.Apply())
	}
	return tup(rollOver(drainEvents(m)), galatui.NoCmd[dashboardMsg]{}.Apply())
}

func tup(m dashboardModel, c galatui.Cmd[dashboardMsg]) std.Tuple[dashboardModel, galatui.Cmd[dashboardMsg]] {
	return std.Tuple[dashboardModel, galatui.Cmd[dashboardMsg]]{
		V1: std.NewImmutable(m),
		V2: std.NewImmutable(c),
	}
}

func drainEvents(m dashboardModel) dashboardModel {
	raw := m.bus.Drain()
	if len(raw) == 0 {
		return m
	}
	logs := m.logs
	for _, e := range raw {
		logs = logs.Info(fmt.Sprintf("%s %s -> %d (%dms)", e.Method, e.Path, e.Status, e.LatencyMs))
	}
	m.logs = logs
	m.perSecond += len(raw)
	m.total += len(raw)
	return m
}

func rollOver(m dashboardModel) dashboardModel {
	nowSec := stdtime.Now().Unix()
	if nowSec == m.lastSecond {
		return m
	}
	// Single-step roll: drop the oldest second, append current.
	steps := int(nowSec - m.lastSecond)
	if steps > len(m.rps) {
		steps = len(m.rps)
	}
	rolled := append([]int{}, m.rps[steps:]...)
	rolled = append(rolled, m.perSecond)
	for i := 1; i < steps; i++ {
		rolled = append(rolled, 0)
	}
	if len(rolled) > 60 {
		rolled = rolled[len(rolled)-60:]
	}
	m.rps = rolled
	m.perSecond = 0
	m.lastSecond = nowSec
	return m
}

// =============================================================================
// View
// =============================================================================

// Layout overhead in rows: header (3) + throughput (8) + footer (1)
// + Border on the log panel (2: top/bottom). Used to size the log tail.
const fixedRowOverhead = 3 + 8 + 1 + 2

func dashboardView(m dashboardModel) galatui.Widget {
	// Tail the log panel to the available height so newest entries are visible.
	// LogPanelView would render every line top-to-bottom and clip newest lines
	// past the viewport — exactly the wrong end. LogPanelViewTail grabs the
	// last n entries instead.
	logTail := galatui.LogPanelViewTail(m.logs, logHeight())

	children := []galatui.LayoutChild{
		galatui.Fixed(3, galatui.Border(headerView(m))),
		galatui.Fixed(8, galatui.Border(throughputView(m))),
		galatui.Flex(1, galatui.Border(logTail)),
		galatui.Fixed(1, footerView()),
	}
	return galatui.Column(collimm.ArrayFromSlice(children))
}

func logHeight() int {
	avail := galatui.TermSize().V2.Get() - fixedRowOverhead
	if avail < 1 {
		return 1
	}
	return avail
}

func headerView(m dashboardModel) galatui.Widget {
	rps := 0
	if len(m.rps) > 0 {
		rps = m.rps[len(m.rps)-1]
	}
	uptime := stdtime.Since(m.startedAt).Round(stdtime.Second).String()
	style := galatui.DefaultStyle().WithBold().WithFg(galatui.BrightCyan())
	text := fmt.Sprintf(" GALA Server Dashboard   %d req/s   %d total   uptime %s", rps, m.total, uptime)
	return galatui.Padding(0, galatui.TextStyled(text, style))
}

func throughputView(m dashboardModel) galatui.Widget {
	children := []galatui.LayoutChild{
		galatui.Fixed(1, galatui.Text(" Throughput (req/s, last 60s)")),
		galatui.Flex(1, galatui.SparklineStyled(
			collimm.ArrayFromSlice(m.rps),
			galatui.DefaultStyle().WithFg(galatui.BrightGreen()),
		)),
	}
	return galatui.Column(collimm.ArrayFromSlice(children))
}

func footerView() galatui.Widget {
	return galatui.TextStyled(" press q to quit", galatui.DefaultStyle().WithDim())
}

// =============================================================================
// Run
// =============================================================================

// RunDashboard starts the TUI loop in the foreground, draining events from the
// bus on each 250ms tick. Blocks until the user presses q or Ctrl+C.
func RunDashboard(b *Bus) dashboardModel {
	program := galatui.Program[dashboardModel, dashboardMsg]{
		Initial: std.NewImmutable(newDashboardModel(b)),
		Update: std.NewImmutable(func(m dashboardModel, msg dashboardMsg) std.Tuple[dashboardModel, galatui.Cmd[dashboardMsg]] {
			return dashboardUpdate(m, msg)
		}),
		View: std.NewImmutable(func(m dashboardModel) galatui.Widget {
			return dashboardView(m)
		}),
	}
	sub := galatui.TickSub[dashboardMsg]{}.Apply(timeutils.Milliseconds(250), func() dashboardMsg {
		return tickMsg()
	})
	return galatui.RunRich[dashboardModel, dashboardMsg](program, func(ev galatui.KeyEvent) dashboardMsg {
		return keyMsg(ev)
	}, sub)
}

// RunWithDashboard is the high-level entry point: it wraps the server with
// TUIFilter, starts Listen() on a background goroutine, and runs the dashboard
// in the foreground. Returns when the user quits the TUI; at that point the
// HTTP-server goroutine is left running and dies on process exit.
func RunWithDashboard(s server.Server) {
	b := NewBus()
	configured := s.WithFilter(TUIFilter(b))

	// Start the HTTP server in the background. We don't share the result —
	// errors from Listen are logged via stderr by the server itself; the TUI
	// owns stdout / the alternate screen.
	var once sync.Once
	go func() {
		_ = once // placeholder so go-vet is happy if we ever extend this
		configured.Listen()
	}()

	_ = RunDashboard(b)
}
