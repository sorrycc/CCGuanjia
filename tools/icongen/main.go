// Command icongen draws CCGuanjia's app icon, a 管 on a tile of the
// macOS icon grid, and writes resources/icon.png:
//
//	go run ./tools/icongen
package main

import (
	"image/png"
	"log"
	"os"

	"github.com/egoist/mygo/ui"
)

func main() {
	view := func(c *ui.Context) {
		c.Root().Background(ui.Transparent)
		ui.Column(c).Fill().Center().Children(func() {
			// The tile of the macOS grid: 824 of 1024 points, rounded 185.
			ui.Box(c).Size(824, 824).Radius(185).
				Gradient(ui.Hex("#2f2f33"), ui.Hex("#131315"), 160).
				InsetShadow(0, 2, 0, 0, ui.RGBA(255, 255, 255, 0.12)).
				Center().Children(func() {
				ui.Column(c).Center().Gap(0).Children(func() {
					ui.Text(c, "管").FontSize(470).FontWeight(800).TextColor(ui.Hex("#d97757"))
					ui.Text(c, "cc").FontSize(120).FontWeight(800).TextColor(ui.Hex("#f5f5f7")).LetterSpacing(6).Margin(-60, 0, 0)
				})
			})
		})
	}
	img := ui.Render(view, 1024, 1024, 1)
	f, err := os.Create("resources/icon.png")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		log.Fatal(err)
	}
}
