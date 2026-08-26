package main

import (
	"image"
	"image/color"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

type C = layout.Context
type D = layout.Dimensions

type UI struct {
	theme *material.Theme
	list  layout.List
	logs  []string
	input widget.Editor
}

func serverUI() {
	w := new(app.Window)
	w.Option(
		app.Title("Jimmy Server Console"),
		app.Size(unit.Dp(700), unit.Dp(450)),
	)

	ui := &UI{
		theme: material.NewTheme(),
		list:  layout.List{Axis: layout.Vertical},
		logs: []string{
			"[INFO] Server UI started",
			"[INFO] Waiting for agents...",
		},
		input: widget.Editor{SingleLine: true, Submit: true}, // Initialize Editor
	}

	var ops op.Ops

	for {
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			// Just return; don't call os.Exit so the HTTP server can keep running.
			return

		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			ui.Layout(gtx)
			e.Frame(gtx.Ops)
		}
	}
}

func (ui *UI) Layout(gtx C) D {
	// Process events from the text input
	for {
		e, ok := ui.input.Update(gtx)
		if !ok {
			break
		}
		if ev, ok := e.(widget.SubmitEvent); ok {
			// Add the typed text to logs
			ui.logs = append(ui.logs, "> "+ev.Text)

			// Link this to your main.go variables
			mutex.Lock()
			userInput = ev.Text
			mutex.Unlock()

			// Clear input box and auto-scroll
			ui.input.SetText("")
			ui.list.Position.BeforeEnd = true
		}
	}

	// Layout the console on top and input on the bottom
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Flexed(1, func(gtx C) D {
			return layout.UniformInset(unit.Dp(16)).Layout(gtx, ui.drawConsoleBox)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx C) D {
				ed := material.Editor(ui.theme, &ui.input, "Type a command...")
				return ed.Layout(gtx)
			})
		}),
	)
}

func (ui *UI) drawConsoleBox(gtx C) D {
	// Background + rounded rect container
	return layout.Stack{}.Layout(gtx,
		// Layer 1: Background & Border
		layout.Expanded(func(gtx C) D {
			bounds := image.Rectangle{Max: gtx.Constraints.Min}
			rr := clip.RRect{
				Rect: bounds,
				NE:   gtx.Dp(8),
				NW:   gtx.Dp(8),
				SE:   gtx.Dp(8),
				SW:   gtx.Dp(8),
			}

			// Dark background
			paint.FillShape(
				gtx.Ops,
				color.NRGBA{R: 18, G: 18, B: 24, A: 255},
				rr.Op(gtx.Ops),
			)

			// Simple border using stroke
			borderColor := color.NRGBA{R: 60, G: 60, B: 70, A: 255}
			borderWidth := float32(gtx.Dp(2))
			paint.FillShape(
				gtx.Ops,
				borderColor,
				clip.Stroke{
					Path:  rr.Path(gtx.Ops),
					Width: borderWidth,
				}.Op(),
			)

			return D{Size: gtx.Constraints.Min}
		}),

		// Layer 2: Scrolling log content
		layout.Stacked(func(gtx C) D {
			return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx C) D {
				return ui.list.Layout(gtx, len(ui.logs), func(gtx C, index int) D {
					lbl := material.Body2(ui.theme, ui.logs[index])
					lbl.Color = color.NRGBA{R: 0, G: 230, B: 140, A: 255}

					return layout.Inset{Bottom: unit.Dp(4)}.Layout(gtx, lbl.Layout)
				})
			})
		}),
	)
}
