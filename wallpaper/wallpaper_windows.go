package wallpaper

import (
	"log"
	"syscall"
	"unsafe"
)

const SPI_SETDESKWALLPAPER = 0x0014
const SPIF_UPDATEINIFILE = 0x01

//Set is function for set wallpaper
func Set(path string) error {
	var mod = syscall.NewLazyDLL("user32.dll")
	var proc = mod.NewProc("SystemParametersInfoW")

	ret, _, _ := proc.Call(
		uintptr(SPI_SETDESKWALLPAPER),
		0,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(path))),
		uintptr(SPIF_UPDATEINIFILE))

	log.Printf("Result code - %d", uint(ret))

	return nil
}
