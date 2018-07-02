package mulberry

import (
	"fmt"
	"image"
	"image/draw"
	"io"
	"os"

	"github.com/oakmound/oak/alg/floatgeom"
	"github.com/oakmound/oak/collision"

	"github.com/oakmound/oak/event"
	"github.com/oakmound/oak/key"
	"github.com/oakmound/oak/mouse"

	"github.com/oakmound/oak/render"
	"github.com/pkg/errors"
)

// A View is a portion of a io.ReadWriteSeeker represented graphically
type View struct {
	cid int

	*render.Sprite
	space    *collision.Space
	buff     io.ReadWriteSeeker
	row, col int64
	// Todo: cursor that will advance col by the width
	// of the active character
	colWidth   int
	lineHeight int
	lineBuffer int
	// x and y are just used during initialization.
	x, y          float64
	width, height int
	mouseOffset   floatgeom.Point2

	lastStartByte int64

	// The byte positions marking the start of each line
	// this slice is precalculated at construction time
	// Todo: Not all of these should be precalculated,
	// as it requires reading the entire buffer. Instead
	// a significant (~1000?) line positions should be tracked
	// at a time, updating in the background as the cursor
	// is moved
	linePositions []int64

	bus  *event.Bus
	font *render.Font

	wordWrap       bool
	lineNumbers    bool
	dirty          bool
	followingMouse bool
}

func defaultView() *View {
	return &View{
		buff:          nil,
		colWidth:      8,
		lineHeight:    12,
		lineBuffer:    1,
		linePositions: make([]int64, 0),
		width:         240,
		height:        240,
		bus:           event.DefaultBus,
		dirty:         true,
		font:          render.DefFont(),
	}
}

// NewFromFile is equivalent to calling `New` on the opened file contents.
func NewFromFile(file string, opts ...Option) (*View, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, errors.Wrap(err, "opening file")
	}
	return New(f, opts...)
}

// New creates a View to display the given ReadWriteSeeker.
func New(r io.ReadWriteSeeker, opts ...Option) (*View, error) {
	v := defaultView()
	for _, o := range opts {
		o(v)
	}
	v.Init()
	v.buff = r
	v.Sprite = render.NewEmptySprite(v.x, v.y, v.width, v.height)
	v.space = collision.NewSpace(v.x, v.y, float64(v.width), float64(v.height), event.CID(v.cid))
	mouse.Add(v.space)
	v.addBindings()
	err := v.precalculateLines()
	return v, err
}

// Init sets up this view's event.CID
func (v *View) Init() event.CID {
	v.cid = int(event.NextID(v))
	return event.CID(v.cid)
}

func (v *View) addBindings() {
	v.bus.Bind(v.moveVert(-1), key.Down+key.UpArrow, v.cid)
	v.bus.Bind(v.moveVert(1), key.Down+key.DownArrow, v.cid)
	v.bus.Bind(v.moveHorz(-1), key.Down+key.LeftArrow, v.cid)
	v.bus.Bind(v.moveHorz(1), key.Down+key.RightArrow, v.cid)

	v.bus.Bind(v.moveVert(-1), key.Held+key.UpArrow, v.cid)
	v.bus.Bind(v.moveVert(1), key.Held+key.DownArrow, v.cid)
	v.bus.Bind(v.moveHorz(-1), key.Held+key.LeftArrow, v.cid)
	v.bus.Bind(v.moveHorz(1), key.Held+key.RightArrow, v.cid)

	v.bus.Bind(v.followMouse, mouse.PressOn, v.cid)
	v.bus.Bind(v.releaseMouse, mouse.Release, v.cid)
}

func (v *View) followMouse(id int, pos interface{}) int {
	me := pos.(mouse.Event)
	v.followingMouse = true
	v.mouseOffset = floatgeom.Point2{
		me.X() - v.X(),
		me.Y() - v.Y(),
	}
	v.bus.Bind(trackMouse, mouse.Drag, v.cid)
	return 0
}

func trackMouse(id int, pos interface{}) int {
	v := event.GetEntity(id).(*View)
	me := pos.(mouse.Event)
	fmt.Println("trackMouse", me)
	diff := me.Sub(v.mouseOffset)
	if v.X() != diff.X() || v.Y() != diff.Y() {
		v.SetPos(diff.X(), diff.Y())
		mouse.UpdateSpace(diff.X(), diff.Y(), float64(v.width), float64(v.height), v.space)
		v.dirty = true
	}
	return 0
}

func (v *View) releaseMouse(id int, _ interface{}) int {
	if v.followingMouse {
		v.bus.UnbindBindable(
			event.UnbindOption{
				BindingOption: event.BindingOption{
					Event: event.Event{
						Name:     mouse.Drag,
						CallerID: v.cid,
					},
					Priority: 0,
				},
				Fn: trackMouse,
			},
		)
		v.followingMouse = false
	}
	return 0
}

func (v *View) precalculateLines() error {
	tmp := make([]byte, 256)
	seen := int64(0)
	v.linePositions = append(v.linePositions, 0)
	for {
		n, err := v.buff.Read(tmp)
		if err != nil && err != io.EOF {
			return err
		}
		for i := 0; i < n; i++ {
			if tmp[i] == '\n' {
				v.linePositions = append(v.linePositions, int64(i)+seen)
			}
		}
		seen += int64(n)

		if err == io.EOF {
			// Add a final fake line
			v.linePositions = append(v.linePositions, seen)
			break
		}
	}
	_, err := v.buff.Seek(0, io.SeekStart)
	return err
}

func (v *View) moveHorz(shift int64) func(int, interface{}) int {
	return func(id int, _ interface{}) int {
		v.col += shift * int64(v.colWidth)
		if v.col < 0 {
			v.col = 0
		}
		// todo: right side column limit
		v.dirty = true
		return 0
	}
}

func (v *View) moveVert(shift int64) func(int, interface{}) int {
	return func(id int, _ interface{}) int {
		v.row += shift
		if v.row < 0 {
			v.row = 0
		}
		if v.row >= int64(len(v.linePositions)) {
			v.row = int64(len(v.linePositions)) - 1
		}
		v.dirty = true
		return 0
	}
}

// GetDims returns the dimensions of this View
func (v *View) GetDims() (int, int) {
	return v.width, v.height
}

// Draw draws this view to the given buffer
func (v *View) Draw(buff draw.Image) {
	v.DrawOffset(buff, 0, 0)
}

// DrawOffset draws this view to the given buffer with some 2D offset
func (v *View) DrawOffset(buff draw.Image, xOff, yOff float64) {
	if v.dirty {
		lineCt := int64(v.height / (v.lineHeight + v.lineBuffer))
		startByte := v.linePositions[v.row]
		endLine := v.row + lineCt + 1
		if endLine >= int64(len(v.linePositions)) {
			endLine = int64(len(v.linePositions)) - 1
			lineCt = (endLine - v.row)
		}
		endByte := v.linePositions[endLine]
		byteCt := endByte - startByte

		v.buff.Seek(startByte, io.SeekStart)
		content := make([]byte, byteCt)
		n, _ := io.ReadFull(v.buff, content)
		if int64(n) != byteCt {
			fmt.Println("Unexpected content length", n, byteCt)
		}
		spRgba := v.Sprite.GetRGBA()
		newRgba := image.NewRGBA(spRgba.Bounds())
		for i := int64(0); i < lineCt; i++ {
			if v.row+i+1 >= int64(len(v.linePositions)) {
				break
			}
			b1 := v.linePositions[v.row+i] - startByte
			b2 := v.linePositions[v.row+i+1] - startByte
			if b2 >= int64(len(content)) {
				b2 = int64(len(content)) - 1
			}
			cut := content[b1:b2]
			txt := v.font.NewStrText(string(cut), 0, 0)
			rgba := txt.ToSprite().GetRGBA()
			bds := rgba.Bounds()
			//render.Draw(txt)
			for x := 0; x < bds.Max.X; x++ {
				for y := 0; y < bds.Max.Y; y++ {
					newRgba.SetRGBA(x, y+int(float64(i)*float64(v.lineHeight+v.lineBuffer)), rgba.RGBAAt(x+int(v.col), y))
				}
			}
		}

		// Read the data from the buffer
		// covering the lines of the contents
		// that fit on the view
		// (based on viewHeight / lineHeight+buffer)
		// and draw those lines to the sprite

		v.Sprite.SetRGBA(newRgba)
		v.lastStartByte = startByte
		v.dirty = false
		v.buff.Seek(0, io.SeekStart)
	}
	v.Sprite.DrawOffset(buff, xOff, yOff)
}
