package services

import (
	"fmt"
	"io"
	"strings"

	go_qr "github.com/piglig/go-qr"
)

const (
	svgFormat string = "svg"
	pngFormat string = "png"

	qrCodeScale  = 20
	qrCodeBorder = 2
)

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
	// WriteQRCode renders a QR code to the writer, taking the same options.
	// Nothing is written to disk, so an arbitrary code cannot leave a file behind.
	WriteQRCode(w io.Writer, content string, options ...QRCodeOption) (err error)
	// WithQRFormat sets the format of the QR code
	// Supported formats are "png" and "svg"
	WithQRFormat(format string) QRCodeOption
	// WithQRForeground sets the foreground color of the QR code
	WithQRForeground(color string) QRCodeOption
	// WithQRBackground sets the background color of the QR code
	WithQRBackground(color string) QRCodeOption
}

type assetGenerator struct{}

func NewAssetGenerator() AssetGenerator {
	return &assetGenerator{}
}

func (s *assetGenerator) CreateQRCodeImage(
	path string,
	content string,
	options ...QRCodeOption,
) error {
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
		if err = qr.PNG(config, path); err != nil {
			return err
		}
	case svgFormat:
		if err = qr.SVG(config, path, defaultOptions.background, defaultOptions.foreground); err != nil {
			return err
		}
	}

	return nil
}

func (s *assetGenerator) WriteQRCode(w io.Writer, content string, options ...QRCodeOption) error {
	opts := &QRCodeOptions{
		format:     pngFormat,
		foreground: "#000000",
		background: "#ffffff",
	}
	for _, o := range options {
		o(opts)
	}

	qr, err := go_qr.EncodeText(content, go_qr.Medium)
	if err != nil {
		return fmt.Errorf("encoding text: %w", err)
	}
	config := go_qr.NewQrCodeImgConfig(qrCodeScale, qrCodeBorder)

	switch opts.format {
	case svgFormat:
		return qr.WriteAsSVG(config, w, opts.background, opts.foreground)
	case pngFormat:
		return qr.WriteAsPNG(config, w)
	default:
		return fmt.Errorf("unsupported format: %s", opts.format)
	}
}
