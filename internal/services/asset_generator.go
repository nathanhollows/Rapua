package services

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
	"github.com/nathanhollows/Rapua/v6/helpers"
	go_qr "github.com/piglig/go-qr"
)

const (
	base10 = 10

	svgFormat string = "svg"
	pngFormat string = "png"

	pageWidth        = 210.0
	pageHeight       = 297.0
	randomCodeLength = 10

	locationNameFontSize = 20.0
	gameNameFontSize     = 28.0

	qrCodeScale  = 20
	qrCodeBorder = 2
)

type PDFPage struct {
	LocationName string
	URL          string
	ImagePath    string
	// []int{R, G, B}
	Background []int
}

type PDFPages []PDFPage

type PDFData struct {
	InstanceName string
	Pages        PDFPages
}

type QRCodeOptions struct {
	format     string
	foreground string
	background string
}

type QRCodeOption func(*QRCodeOptions)

func (*assetGenerator) WithQRFormat(format string) QRCodeOption {
	return func(o *QRCodeOptions) {
		o.format = strings.ToLower(format)
	}
}

func (*assetGenerator) WithQRForeground(color string) QRCodeOption {
	return func(o *QRCodeOptions) {
		o.foreground = color
	}
}

func (*assetGenerator) WithQRBackground(color string) QRCodeOption {
	return func(o *QRCodeOptions) {
		o.background = color
	}
}

type AssetGenerator interface {
	// CreateQRCodeImage creates a QR code image with the given options
	// Supported options are:
	// - WithQRFormat(format string), where format is "png" or "svg"
	// - WithForeground(color string), where color is a hex color code
	// - WithBackground(color string), where color is a hex color code
	CreateQRCodeImage(path string, content string, options ...QRCodeOption) (err error)
	// WithQRFormat sets the format of the QR code
	// Supported formats are "png" and "svg"
	WithQRFormat(format string) QRCodeOption
	// WithQRForeground sets the foreground color of the QR code
	WithQRForeground(color string) QRCodeOption
	// WithQRBackground sets the background color of the QR code
	WithQRBackground(color string) QRCodeOption

	// CreateArchive creates a zip archive from the given paths
	// Returns the path to the archive
	// Accepts a list of paths to files to add to the archive
	// Accepts an optional list of filenames to use for the files in the archive
	CreateArchive(paths []string) (path string, err error)
	// CreatePDF creates a PDF document from the given data
	// Returns the path to the PDF
	CreatePDF(data PDFData) (string, error)
	// GetQRCodePathAndContent returns the path and content for a QR code
	GetQRCodePathAndContent(action, id, name, extension string) (string, string)
}

type assetGenerator struct{}

func NewAssetGenerator() AssetGenerator {
	return &assetGenerator{}
}

func (s *assetGenerator) CreateQRCodeImage(
	path string,
	content string,
	options ...QRCodeOption,
) (err error) {
	defaultOptions := &QRCodeOptions{
		format:     pngFormat,
		foreground: "#000000",
		background: "#ffffff",
	}

	// Apply each option to the default options
	for _, o := range options {
		o(defaultOptions)
	}

	// Validate the options
	if defaultOptions.format != pngFormat && defaultOptions.format != svgFormat {
		return fmt.Errorf("unsupported format: %s", defaultOptions.format)
	}

	qr, err := go_qr.EncodeText(content, go_qr.Medium)
	if err != nil {
		return fmt.Errorf("encoding text: %w", err)
	}
	config := go_qr.NewQrCodeImgConfig(qrCodeScale, qrCodeBorder)

	switch defaultOptions.format {
	case pngFormat:
		err := qr.PNG(config, path)
		if err != nil {
			return err
		}
	case svgFormat:
		err := qr.SVG(config, path, defaultOptions.background, defaultOptions.foreground)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *assetGenerator) CreateArchive(paths []string) (path string, err error) {
	// Create the file
	path = "assets/codes/" + helpers.NewCode(
		randomCodeLength,
	) + "-" + strconv.FormatInt(
		time.Now().UnixNano(),

		base10,
	) + ".zip"
	archive, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("could not create archive: %w", err)
	}
	defer archive.Close()

	zipWriter := zip.NewWriter(archive)
	defer zipWriter.Close()

	// Add each file to the zip
	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			return "", err
		}
		defer file.Close()

		info, err := file.Stat()
		if err != nil {
			return "", err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return "", err
		}

		header.Name = strings.TrimPrefix(path, "assets/codes/")
		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return "", err
		}

		_, err = io.Copy(writer, file)
		if err != nil {
			return "", err
		}
	}

	return path, nil
}

func (s *assetGenerator) CreatePDF(data PDFData) (path string, err error) {
	// Set up the document
	pdf := fpdf.New(fpdf.OrientationPortrait, fpdf.UnitMillimeter, fpdf.PageSizeA4, "")
	pdf.AddUTF8Font("ArchivoBlack", "", "./assets/fonts/ArchivoBlack-Regular.ttf")
	pdf.AddUTF8Font("OpenSans", "", "./assets/fonts/OpenSans.ttf")

	// Add pages
	for _, page := range data.Pages {
		s.addPage(pdf, page, data.InstanceName)
	}

	path = "assets/codes/" + helpers.NewCode(
		randomCodeLength,
	) + "-" + strconv.FormatInt(
		time.Now().UnixNano(),
		base10,
	) + ".pdf"
	err = pdf.OutputFileAndClose(path)
	if err != nil {
		return "", err
	}

	return path, nil
}

func (s *assetGenerator) addPage(pdf *fpdf.Fpdf, page PDFPage, instanceName string) {
	pdf.AddPage()
	// Set the background color
	if len(page.Background) == 3 {
		pdf.SetFillColor(page.Background[0], page.Background[1], page.Background[2])
		pdf.Rect(0, 0, pageWidth, pageHeight, "F")
	}

	// Add the instance name
	pdf.SetFont("ArchivoBlack", "", gameNameFontSize)
	title := strings.ToUpper(instanceName)
	pdf.SetY(32)
	//nolint:mnd // centered
	pdf.SetX((pageWidth - pdf.GetStringWidth(title)) / 2)
	pdf.Cell(130, 32, title)

	// Add the location name
	pdf.SetFont("OpenSans", "", locationNameFontSize)
	pdf.SetY(40)
	//nolint:mnd // centered
	pdf.SetX((pageWidth - pdf.GetStringWidth(page.LocationName)) / 2)
	pdf.Cell(40, 70, page.LocationName)

	// Add the QR code
	if page.ImagePath[len(page.ImagePath)-3:] == pngFormat {
		pdf.Image(page.ImagePath, 50, 90, 110, 110, false, "", 0, "")
	}

	// Render the URL
	scanText := page.URL
	scanText = strings.ReplaceAll(scanText, "https://", "")
	scanText = strings.ReplaceAll(scanText, "http://", "")
	scanText = strings.ReplaceAll(scanText, "www.", "")
	pdf.SetY(180)
	//nolint:mnd // centered
	pdf.SetX((pageWidth - pdf.GetStringWidth(scanText)) / 2)
	pdf.Cell(40, 70, scanText)
}

func (s *assetGenerator) GetQRCodePathAndContent(action, id, name, extension string) (string, string) {
	content := os.Getenv("SITE_URL")
	path := "assets/codes/"
	name = strings.Trim(name, " ")
	re := regexp.MustCompile(`[^\d\p{Latin} -]`)
	name = re.ReplaceAllString(name, "")
	content = content + "/s/" + id
	path = path + extension + "/" + id + " " + name + "." + extension
	return path, content
}
