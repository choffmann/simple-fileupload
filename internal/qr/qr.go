package qr

import qrcode "github.com/skip2/go-qrcode"

func Generate(url string, size int) ([]byte, error) {
	return qrcode.Encode(url, qrcode.Medium, size)
}
