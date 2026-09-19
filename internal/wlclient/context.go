package client

import (
	"errors"
	"fmt"
	"net"
	"os"
	"time"
)

type Context struct {
	conn      *net.UnixConn
	objects   map[uint32]Proxy
	currentID uint32
}

func (ctx *Context) Register(p Proxy) {
	ctx.currentID++
	p.SetID(ctx.currentID)
	p.SetContext(ctx)
	ctx.objects[ctx.currentID] = p
}

// RegisterWithID registers a proxy under an ID chosen by the *server*, as
// opposed to Register (which allocates the next client-side ID). This is
// needed for protocol events whose argument type is new_id: unlike a new_id
// argument on a *request* (where the client picks the ID), a new_id sent by
// the compositor inside an *event* carries the object ID it already
// allocated, and the client must simply start tracking that exact ID.
//
// This is an addition on top of upstream github.com/rajveermalviya/go-wayland
// (vendored here under internal/wlclient, BSD-2-Clause, see
// LICENSE.go-wayland) needed to correctly implement
// zwlr_output_manager_v1.head and zwlr_output_head_v1.mode, both of which are
// new_id-typed event arguments.
func (ctx *Context) RegisterWithID(id uint32, p Proxy) {
	p.SetID(id)
	p.SetContext(ctx)
	ctx.objects[id] = p
}

func (ctx *Context) Unregister(p Proxy) {
	delete(ctx.objects, p.ID())
}

func (ctx *Context) GetProxy(id uint32) Proxy {
	return ctx.objects[id]
}

func (ctx *Context) Close() error {
	return ctx.conn.Close()
}

// SetDeadline sets a read/write deadline on the underlying connection, so
// callers can bound how long they wait for compositor responses. This is an
// addition on top of upstream go-wayland (see the note on RegisterWithID).
func (ctx *Context) SetDeadline(t time.Time) error {
	return ctx.conn.SetDeadline(t)
}

func (ctx *Context) Dispatch() error {
	senderID, opcode, fd, data, err := ctx.ReadMsg()
	if err != nil {
		return fmt.Errorf("ctx.Dispatch: unable to read msg: %w", err)
	}

	sender, ok := ctx.objects[senderID]
	if ok {
		if sender, ok := sender.(Dispatcher); ok {
			sender.Dispatch(opcode, fd, data)
		} else {
			return fmt.Errorf("ctx.Dispatch: sender doesn't implement Dispatch method (senderID=%d)", senderID)
		}
	} else {
		return fmt.Errorf("ctx.Dispatch: unable find sender (senderID=%d)", senderID)
	}

	return nil
}

func Connect(addr string) (*Display, error) {
	if addr == "" {
		runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
		if runtimeDir == "" {
			return nil, errors.New("env XDG_RUNTIME_DIR not set")
		}
		if addr == "" {
			addr = os.Getenv("WAYLAND_DISPLAY")
		}
		if addr == "" {
			addr = "wayland-0"
		}
		addr = runtimeDir + "/" + addr
	}

	ctx := &Context{
		objects: map[uint32]Proxy{},
	}

	conn, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: addr, Net: "unix"})
	if err != nil {
		return nil, err
	}
	ctx.conn = conn

	return NewDisplay(ctx), nil
}
