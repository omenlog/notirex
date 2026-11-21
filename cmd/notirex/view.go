package main

import (
	"math"
	"strings"
	"time"

	"notirex/internal/notification"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type notificationRenderPhase byte

const (
	notificationPhaseMeasure notificationRenderPhase = iota
	notificationPhaseDraw
)

const (
	defaultDuration = 2 * time.Second

	minimumWindowWidth  = 180
	maximumWindowWidth  = 540
	minimumWindowHeight = 40
	maximumTextWidth    = 480
	textWidthSafety     = 4
	iconSize            = 16
	iconRadius          = 5
	contentGap          = 10
	horizontalPadding   = 14
	verticalPadding     = 12
	pillRadius          = 20
	screenMargin        = 20
)

func clamp(value, minimum, maximum int) int {
	return max(minimum, min(maximum, value))
}

func notificationWindowSize(area mygo.Rectangle, width, height int) (int, int) {
	if area.Width > 0 {
		width = min(width, area.Width)
	}
	if area.Height > 0 {
		height = min(height, area.Height)
	}

	return width, height
}

type notificationView struct {
	notice        notification.Notification
	window        *mygo.Window
	workArea      mygo.Rectangle
	duration      time.Duration
	phase         notificationRenderPhase
	singleLine    bool
	showScheduled bool
}

func (v *notificationView) getDimensions(textWidth, textHeight float32) (int, int) {
	iconWidth := 0
	gap := 0
	safety := textWidthSafety
	minimumWidth := 1
	if v.notice.Appearance().Icon != "" {
		iconWidth = iconSize
		gap = contentGap
		minimumWidth = minimumWindowWidth
	}

	width := horizontalPadding*2 + iconWidth + gap + safety + int(math.Ceil(float64(textWidth)))
	height := verticalPadding*2 + max(iconWidth, int(math.Ceil(float64(textHeight))))
	return clamp(width, minimumWidth, maximumWindowWidth), max(height, minimumWindowHeight)
}

func (v *notificationView) measureSize(c *ui.Context) (int, int) {
	message := v.notice.DisplayMessage()
	span := ui.Span{Text: message, Size: 13, Weight: 400}
	naturalWidth, _ := c.MeasureText(0, span)
	v.singleLine = !strings.ContainsRune(message, '\n') && naturalWidth <= maximumTextWidth
	textWidth := min(naturalWidth, maximumTextWidth)
	_, textHeight := c.MeasureText(textWidth, span)
	width, height := v.getDimensions(textWidth, textHeight)
	width, height = notificationWindowSize(v.workArea, width, height)

	return width, height
}

func (v *notificationView) getPosition(width, height int) (int, int) {
	screen := v.workArea
	position := v.notice.Position

	if position == notification.TopRight {
		x := screen.X + screen.Width - width - screenMargin
		y := screen.Y + screenMargin
		if x < screen.X {
			x = screen.X
		}
		if y+height > screen.Y+screen.Height {
			y = max(screen.Y, screen.Y+screen.Height-height)
		}
		return x, y
	}

	if position == notification.TopLeft {
		x := screen.X + screenMargin
		y := screen.Y + screenMargin
		if x+width > screen.X+screen.Width {
			x = max(screen.X, screen.X+screen.Width-width)
		}
		if y+height > screen.Y+screen.Height {
			y = max(screen.Y, screen.Y+screen.Height-height)
		}
		return x, y
	}

	if position == notification.BottomLeft {
		x := screen.X + screenMargin
		y := screen.Y + screen.Height - height - screenMargin
		if x+width > screen.X+screen.Width {
			x = max(screen.X, screen.X+screen.Width-width)
		}
		if y < screen.Y {
			y = screen.Y
		}

		return x, y
	}

	if position == notification.BottomRight {
		x := screen.X + screen.Width - width - screenMargin
		if x < screen.X {
			x = screen.X
		}
		y := screen.Y + screen.Height - height - screenMargin
		if y < screen.Y {
			y = screen.Y
		}

		return x, y
	}

	return 0, 0
}

func (v *notificationView) show(c *ui.Context) {
	look := v.notice.Appearance()
	accent := ui.Hex(look.Color)
	base := ui.Hex("#0a0d10")
	bgAccent := 0.01

	container := ui.Row(c).
		Fill().
		Padding(verticalPadding, horizontalPadding).
		Gap(contentGap).
		AlignItems(ui.Center).
		Radius(pillRadius).
		Background(base.Mix(accent, float32(bgAccent)).Alpha(0.9)).
		Border(0.5, accent.Alpha(0.1))

	container.Children(func() {
		if look.Icon != "" {
			ui.Box(c).
				Size(iconSize, iconSize).
				Shrink(0).
				Radius(iconRadius).
				Center().
				Background(accent).
				Children(func() {
					ui.Text(c, look.Icon).
						FontSize(11).
						Bold().
						TextColor(base)
				})
		}

		message := ui.Text(c, v.notice.DisplayMessage()).
			Grow(1).
			FontSize(13).
			FontWeight(400).
			TextColor(accent)

		if v.singleLine {
			message.SingleLine()
		}
	})
}

func (v *notificationView) render(c *ui.Context) {
	c.Root().Background(ui.Transparent)

	if v.phase == notificationPhaseMeasure {
		width, height := v.measureSize(c)
		x, y := v.getPosition(width, height)

		// Move to the drawing phase before changing the window geometry, since
		// either operation may cause MyGo to request another frame.
		v.phase = notificationPhaseDraw
		v.window.SetSize(width, height)
		v.window.SetPosition(x, y)
		return
	}

	v.show(c)
	v.scheduleShow()
}

func (v *notificationView) scheduleShow() {
	if v.showScheduled {
		return
	}
	v.showScheduled = true

	// Posting from a goroutine reveals the already-visible window on the next
	// main-loop turn, after the drawing-phase render has completed.
	go func() {
		mygo.RunOnMain(func() {
			v.window.SetOpacity(1)
		})
		time.Sleep(v.duration)
		v.window.Close()
	}()
}

func runNativeNotification(notice notification.Notification, duration time.Duration) error {
	mygo.App.SetName("notirex")
	mygo.App.SetActivationPolicy(mygo.ActivationPolicyAccessory)
	mygo.App.WhenReady(func() {
		display := mygo.Screen.DisplayNearestPoint(mygo.Screen.CursorScreenPoint())
		view := &notificationView{
			notice:   notice,
			workArea: display.WorkArea,
			duration: duration,
		}

		view.window = mygo.NewWindow(mygo.WindowOptions{
			Title:             "notirex",
			X:                 display.WorkArea.X,
			Y:                 display.WorkArea.Y,
			Width:             1,
			Height:            1,
			Hidden:            true,
			Panel:             true,
			Frameless:         true,
			Transparent:       true,
			Vibrancy:          mygo.VibrancyHUD,
			CornerRadius:      pillRadius,
			IgnoreMouseEvents: true,
			DisableResize:     true,
			DisableMove:       true,
			DisableClose:      true,
			Content:           ui.View(view.render),
		})
		view.window.SetOpacity(0)
		view.window.OnReadyToShow(func() {
			// Hidden native UI does not draw another frame after becoming ready.
			// Show the final-sized window invisibly so MyGo draws its content.
			view.window.ShowInactive()
			view.window.Invalidate()
		})
	})

	return mygo.App.Run()
}
