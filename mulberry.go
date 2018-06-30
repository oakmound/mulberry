package mulberry

import (
	"image/draw"
	"io"
	"os"

	"github.com/oakmound/oak/render"
	"github.com/pkg/errors"
)

type View struct {
	render.LayeredPoint
	buff       io.ReadWriteSeeker
	lineHeight int
	lineBuffer int
	width      int
	height     int
}

func NewFromFile(file string) (*View, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, errors.Wrap(err, "opening file")
	}
	return New(f, 0, 0), nil
}

func New(r io.ReadWriteSeeker, w, h int) *View {
	return &View{
		render.NewLayeredPoint(0, 0, 0),
		r,
		12,
		1,
		w,
		h,
	}
}

func (v *View) GetDims() (int, int) {
	return v.width, v.height
}

func (v *View) Draw(buff draw.Image) {
	v.DrawOffset(buff, 0, 0)
}

func (v *View) DrawOffset(buff draw.Image, xOff, yOff int) {

	v.Sprite.DrawOffset(buff, xOff, yOff)
}
