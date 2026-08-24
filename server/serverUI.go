package main

import (
	"image"
	"image/color"
	"os"

	"gioui.org/app"
	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

type C = layout.Context
type D = layout.Dimensions

type UI struct {
	theme *material.Theme
	list  layout.List
	logs  []string
}

func serverUI() {
	go func() {
		w := new(app.Window)
		w.Option(app.Title("Console Output Box"))
		w.Option(app.Size(unit.Dp(500), unit.Dp(300)))

		ui := &UI{
			theme: material.NewTheme(),
			list:  layout.List{Axis: layout.Vertical},
			logs: []string{
				"[10:00:01] System booted.",
				"[10:00:02] Loading modules...",
				"[10:00:03] Connected to local database.",
				"[10:00:05] WARNING: CPU temperature elevated.",
				"[10:00:10] Task completed successfully.",
			},
		}

		ui.theme.Shaper = gofont.Collection()

		var ops op.Ops
		for {
			switch e := w.Event().(type) {
			case app.DestroyEvent:
				os.Exit(0)
			case app.FrameEvent:
				gtx := app.NewContext(&ops, e)
				ui.Layout(gtx)
				e.Frame(gtx.Ops)
			}
		}
	}()
	app.Main()
}

func (ui *UI) Layout(gtx C) D {
	// Add padding around the main window outer bounds
	return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx C) D {
		// Render the console container box
		return ui.drawConsoleBox(gtx)
	})
}

func (ui *UI) drawConsoleBox(gtx C) D {
	// Wrap the entire list within a fixed height or expandable box area
	return layout.Stack{}.Layout(gtx,
		// Layer 1: Background & Border
		layout.Expanded(func(gtx C) D {
			bounds := imageRect(gtx.Constraints.Min)
			rr := clip.RRect{
				Rect: bounds,
				NE:   gtx.Dp(4), NW: gtx.Dp(4), SE: gtx.Dp(4), SW: gtx.Dp(4),
			}
			// Fill dark background
			paint.FillShape(gtx.Ops, color.NRGBA{R: 20, G: 20, B: 20, A: 255}, rr.Op(gtx.Ops))

			// Draw border outline
			borderThickness := gtx.Dp(1)
			return widgetBorder(color.NRGBA{R: 70, G: 70, B: 70, A: 255}, borderThickness, rr).Layout(gtx)
		}),
		// Layer 2: Scrolling Content
		layout.Stacked(func(gtx C) D {
			return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx C) D {
				return ui.list.Layout(gtx, len(ui.logs), func(gtx C, index int) D {
					// Render log line using monospace font styling
					lbl := material.Body2(ui.theme, ui.logs[index])
					lbl.Font.Typeface = "Go Mono"
					lbl.Color = color.NRGBA{R: 0, G: 230, B: 120, A: 255} // Console green text

					return layout.Inset{Bottom: unit.Dp(4)}.Layout(gtx, lbl.Layout)
				})
			})
		}),
	)
}

// Helper to construct background rect bounds
func imageRect(pt image.Point) clip.SimpleRRect {
	return clip.RRect{Rect: image.Rectangle{Max: pt}}
}

// Helper to draw a border outline stroke around an RRect clip area
type widgetBorder struct {
	Color  color.NRGBA
	Width  int
	Corner clip.RRect
}

func (b widgetBorder) Layout(gtx C) D {
	paint.FillShape(gtx.Ops, b.Color, clip.Stroke{
		Path:  b.Corner.Path(gtx.Ops),
		Width: float32(b.Width),
	}.Op())
	return D{Size: gtx.Constraints.Min}
}
