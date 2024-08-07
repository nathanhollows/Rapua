package models

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/nathanhollows/Rapua/internal/helpers"
	"github.com/nathanhollows/Rapua/pkg/db"
	qrcode "github.com/yeqown/go-qrcode/v2"
	qrwriter "github.com/yeqown/go-qrcode/writer/standard"
)

type Marker struct {
	baseModel

	Code         string    `bun:",unique,pk" json:"code"`
	Lat          float64   `bun:",type:float" json:"lat"`
	Lng          float64   `bun:",type:float" json:"lng"`
	Name         string    `bun:",type:varchar(255)" json:"name"`
	TotalVisits  int       `bun:",type:int" json:"total_visits"`
	CurrentCount int       `bun:",type:int" json:"current_count"`
	AvgDuration  float64   `bun:",type:float" json:"avg_duration"`
	Locations    Locations `bun:"rel:has-many,join:code=marker_id" json:"locations"`
}

type Markers []*Marker

// Save saves or updates a location
func (l *Marker) Save(ctx context.Context) error {
	insert := false
	var err error
	if l.Code == "" {
		l.Code = helpers.NewCode(5)
		insert = true
	}

	if insert {
		_, err = db.DB.NewInsert().Model(l).Exec(ctx)
	} else {
		_, err = db.DB.NewUpdate().Model(l).WherePK("code").Exec(ctx)
	}
	if err != nil {
		log.Error(err)
	}
	return err
}

func (l *Marker) GenerateQRCode() error {
	// Only generate the QR code if it doesn't exist
	if l.checkQRPath(true) && l.checkQRPath(false) {
		return nil
	}

	qrc, err := l.generateQRCode(true)
	if err != nil {
		return err
	}

	if err := saveQRCode(qrc, l.getQRPath(true)); err != nil {
		return err
	}

	// Commented out for now
	// This is for the scan out QR code
	// if l.MustScanOut {
	// 	qrc, err := l.generateQRCode(false)
	// 	if err != nil {
	// 		return err
	// 	}
	//
	// 	if err := saveQRCode(qrc, l.getQRPath(false)); err != nil {
	// 		return err
	// 	}
	// }

	return nil
}

// FindMarkerByCode returns a marker by code
func FindMarkerByCode(ctx context.Context, code string) (*Marker, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	var marker Marker
	err := db.DB.NewSelect().Model(&marker).Where("code = ?", code).Scan(ctx)
	return &marker, err
}

func (l *Marker) getScanURL(scanningIn bool) string {
	var url string
	if scanningIn {
		url = os.Getenv("SITE_URL") + "/s/" + l.Code
	} else {
		url = os.Getenv("SITE_URL") + "/o/" + l.Code
	}
	return url
}

func (l *Marker) getQRFilename(scanningIn bool) string {
	var path string
	if scanningIn {
		path = l.Code + " " + l.Name + " in.png"
	} else {
		path = l.Code + " " + l.Name + " out.png"
	}
	return path
}

func (l *Marker) getQRPath(scanningIn bool) string {
	return "./assets/codes/" + l.getQRFilename(scanningIn)
}

func (l *Marker) checkQRPath(scanningIn bool) bool {
	_, err := os.Stat(l.getQRPath(scanningIn))
	return err == nil
}

func (l *Marker) generateQRCode(scanningIn bool) (*qrcode.QRCode, error) {
	url := l.getScanURL(scanningIn)
	qrc, err := qrcode.New(url)
	if err != nil {
		fmt.Printf("could not generate QRCode: %v", err)
		return nil, err
	} else {
		return qrc, nil
	}
}

func saveQRCode(qrc *qrcode.QRCode, path string) error {
	w, err := qrwriter.New(
		path,
		qrwriter.WithBgTransparent(),
		qrwriter.WithBuiltinImageEncoder(qrwriter.PNG_FORMAT))
	if err != nil {
		fmt.Printf("could not generate QRCode: %v", err)
		return err
	}

	err = qrc.Save(w)
	if err != nil {
		fmt.Printf("could not generate QRCode: %v", err)
		return err
	} else {
		return nil
	}
}

// SetCoords sets the latitude and longitude of the location
func (l *Marker) SetCoords(lat, lng float64) error {
	l.Lat = lat
	l.Lng = lng
	return nil
}

// // GeneratePoster pre-emptively generates the poster for the new clue
// func (c Clue) GeneratePoster() error {
// 	imgb, _ := os.Open("assets/poster.png")
// 	img, _ := png.Decode(imgb)
// 	defer imgb.Close()

// 	background := color.RGBA{255, 213, 79, 255}
// 	foreground := color.RGBA{35, 35, 35, 255}
// 	// TODO: Factor out the hard coded link
// 	qrc, err := qrcode.New("https://trace.co.nz/"+c.Code,
// 		qrcode.WithBgColor(background),
// 		qrcode.WithFgColor(foreground),
// 		qrcode.WithBuiltinImageEncoder(qrcode.PNG_FORMAT))
// 	if err != nil {
// 		fmt.Printf("could not generate QRCode: %v", err)
// 		return err
// 	}
// 	if err := qrc.Save("assets/img/temp/" + c.Code + ".png"); err != nil {
// 		fmt.Printf("could not save image: %v", err)
// 		return err
// 	}

// 	wmb, _ := os.Open("assets/img/temp/" + c.Code + ".png")
// 	watermark, _ := png.Decode(wmb)
// 	defer wmb.Close()

// 	offset := image.Pt(463, 1075)
// 	b := img.Bounds()
// 	m := image.NewRGBA(b)
// 	draw.Draw(m, b, img, image.Point{}, draw.Src)
// 	draw.Draw(m, watermark.Bounds().Add(offset), watermark, image.Point{}, draw.Over)

// 	addLabel(m, 440, 2050, fmt.Sprint("trace.co.nz/", c.Code))

// 	imgw, _ := os.Create("assets/img/posters/" + c.Code + ".png")
// 	png.Encode(imgw, m)
// 	defer imgw.Close()

// 	os.Remove("assets/img/temp/" + c.Code + ".png")

// 	return nil
// }

// var (
// 	dpi      = flag.Float64("dpi", 72, "screen resolution in Dots Per Inch")
// 	fontfile = flag.String("fontfile", "assets/fonts/RobotoMono-Bold.ttf", "RobotoMono-Bold")
// 	hinting  = flag.String("hinting", "none", "none | full")
// 	size     = flag.Float64("size", 72, "font size in points")
// )

// func addLabel(img *image.RGBA, x, y int, label string) {
// 	flag.Parse()
// 	col := color.RGBA{254, 214, 79, 255}
// 	point := fixed.Point26_6{X: fixed.Int26_6(x * 64), Y: fixed.Int26_6(y * 64)}

// 	// Read the font data.
// 	fontBytes, err := ioutil.ReadFile(*fontfile)
// 	if err != nil {
// 		log.Println(err)
// 		return
// 	}
// 	f, err := truetype.Parse(fontBytes)
// 	if err != nil {
// 		log.Println(err)
// 		return
// 	}
// 	// Draw the text.
// 	h := font.HintingNone
// 	switch *hinting {
// 	case "full":
// 		h = font.HintingFull
// 	}
// 	d := &font.Drawer{
// 		Dst: img,
// 		Src: image.NewUniform(col),
// 		Face: truetype.NewFace(f, &truetype.Options{
// 			Size:    *size,
// 			DPI:     *dpi,
// 			Hinting: h,
// 		}),
// 		Dot: point,
// 	}
// 	d.DrawString(label)
// }
