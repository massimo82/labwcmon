package client

import (
	"fmt"
	"unsafe"
)

func (ctx *Context) WriteMsg(b []byte, oob []byte) error {
	n, oobn, err := ctx.conn.WriteMsgUnix(b, oob, nil)
	if err != nil {
		return err
	}
	if n != len(b) || oobn != len(oob) {
		return fmt.Errorf("ctx.WriteMsg: incorrect number of bytes written (n=%d oobn=%d)", n, oobn)
	}

	return nil
}

func PutUint32(dst []byte, v uint32) {
	_ = dst[3]
	*(*uint32)(unsafe.Pointer(&dst[0])) = v
}

func PutFixed(dst []byte, f float64) {
	fx := fixedFromfloat64(f)
	_ = dst[3]
	*(*int32)(unsafe.Pointer(&dst[0])) = fx
}

// PutString writes a Wayland wire "string" argument: a 32-bit length
// (the true string length, including the NUL terminator - NOT padded to a
// 4-byte boundary) followed by the string bytes, a NUL terminator, and
// zero-padding up to a 4-byte boundary.
//
// dst must be pre-sized by the caller to the full padded slot
// (4 + PaddedLen(len(v)+1)); l is the caller's padded slot size, used only
// historically to size dst and otherwise unused here.
//
// This is a bugfix on top of upstream github.com/rajveermalviya/go-wayland
// (vendored here under internal/wlclient, BSD-2-Clause): every generated
// call site passes the *padded* length here, which upstream then also wrote
// into the wire length field. Most compositors are lenient and just scan
// for the NUL terminator, so this went unnoticed, but strict ones (labwc
// among them) validate the declared length and reject the request outright
// with "invalid arguments" - which is exactly what broke monitor detection
// here. The fix: always write the true (unpadded) length, regardless of
// what padded size the caller passed in for buffer-slicing purposes.
func PutString(dst []byte, v string, l int) {
	PutUint32(dst[:4], uint32(len(v)+1))
	copy(dst[4:], []byte(v))
}

func PutArray(dst []byte, a []byte) {
	PutUint32(dst[:4], uint32(len(a)))
	copy(dst[4:], a)
}
