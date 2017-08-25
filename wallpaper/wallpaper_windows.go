package wallpaper

import (
	"syscall"
	"unsafe"
)

//Set is function for set wallpaper
func Set(path string) error {
	var mod = syscall.NewLazyDLL("user32.dll")
	var proc = mod.NewProc("SystemParametersInfoW")
	var SPI_SETDESKWALLPAPER = 0x0014
	var SPIF_UPDATEINIFILE = 0x01

	proc.Call(
		uintptr(SPI_SETDESKWALLPAPER),
		0,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(path))),
		uintptr(SPIF_UPDATEINIFILE))

	return nil
}
