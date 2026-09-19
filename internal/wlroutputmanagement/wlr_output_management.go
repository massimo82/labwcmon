// Package wlroutputmanagement is a hand-written, pure-Go client binding for
// the wlr-output-management-unstable-v1 Wayland protocol.
//
// It's written in the same style go-wayland-scanner would generate (see
// https://github.com/rajveermalviya/go-wayland), against the same
// wayland/client runtime, but is checked in directly instead of code-generated
// because the generator's own dependency (golang.org/x/tools) can't be
// fetched in every build environment. If you do have unrestricted network
// access and want to regenerate/verify this file against upstream, run:
//
//	go run github.com/rajveermalviya/go-wayland/cmd/go-wayland-scanner \
//	  -pkg wlroutputmanagement -prefix zwlr -suffix v1 \
//	  -o wlr_output_management.go \
//	  -i https://raw.githubusercontent.com/emersion/wlr-randr/master/protocol/wlr-output-management-unstable-v1.xml
//
// Protocol XML Copyright © 2019 Purism SPC (MIT-style permissive license,
// see the license text embedded in the upstream XML).
package wlroutputmanagement

import client "labwcmon/internal/wlclient"

// ---------------------------------------------------------------------------
// zwlr_output_manager_v1
// ---------------------------------------------------------------------------

// OutputManager is a manager that allows reading and writing the current
// output device configuration.
type OutputManager struct {
	client.BaseProxy
	headHandler     OutputManagerHeadHandlerFunc
	doneHandler     OutputManagerDoneHandlerFunc
	finishedHandler OutputManagerFinishedHandlerFunc
}

func NewOutputManager(ctx *client.Context) *OutputManager {
	m := &OutputManager{}
	ctx.Register(m)
	return m
}

// CreateConfiguration creates a new output configuration object that can be
// used to enable/disable/reconfigure heads and then apply or test the result.
func (i *OutputManager) CreateConfiguration(serial uint32) (*OutputConfiguration, error) {
	id := NewOutputConfiguration(i.Context())
	const opcode = 0
	const _reqBufLen = 8 + 4 + 4
	var _reqBuf [_reqBufLen]byte
	l := 0
	client.PutUint32(_reqBuf[l:4], i.ID())
	l += 4
	client.PutUint32(_reqBuf[l:l+4], uint32(_reqBufLen<<16|opcode&0x0000ffff))
	l += 4
	client.PutUint32(_reqBuf[l:l+4], id.ID())
	l += 4
	client.PutUint32(_reqBuf[l:l+4], serial)
	l += 4
	err := i.Context().WriteMsg(_reqBuf[:], nil)
	return id, err
}

// Stop tells the compositor the client no longer wants manager events.
func (i *OutputManager) Stop() error {
	const opcode = 1
	const _reqBufLen = 8
	var _reqBuf [_reqBufLen]byte
	l := 0
	client.PutUint32(_reqBuf[l:4], i.ID())
	l += 4
	client.PutUint32(_reqBuf[l:l+4], uint32(_reqBufLen<<16|opcode&0x0000ffff))
	l += 4
	err := i.Context().WriteMsg(_reqBuf[:], nil)
	return err
}

// OutputManagerHeadEvent introduces a new head (monitor).
type OutputManagerHeadEvent struct {
	Head *OutputHead
}
type OutputManagerHeadHandlerFunc func(OutputManagerHeadEvent)

func (i *OutputManager) SetHeadHandler(f OutputManagerHeadHandlerFunc) {
	i.headHandler = f
}

// OutputManagerDoneEvent is sent once all pending head/mode changes for the
// current configuration snapshot have been sent. Carries the serial to use
// in a subsequent CreateConfiguration call.
type OutputManagerDoneEvent struct {
	Serial uint32
}
type OutputManagerDoneHandlerFunc func(OutputManagerDoneEvent)

func (i *OutputManager) SetDoneHandler(f OutputManagerDoneHandlerFunc) {
	i.doneHandler = f
}

type OutputManagerFinishedEvent struct{}
type OutputManagerFinishedHandlerFunc func(OutputManagerFinishedEvent)

func (i *OutputManager) SetFinishedHandler(f OutputManagerFinishedHandlerFunc) {
	i.finishedHandler = f
}

func (i *OutputManager) Dispatch(opcode uint32, fd int, data []byte) {
	switch opcode {
	case 0: // head
		l := 0
		id := client.Uint32(data[l : l+4])
		l += 4
		head := NewOutputHead(i.Context())
		i.Context().RegisterWithID(id, head)
		if i.headHandler != nil {
			i.headHandler(OutputManagerHeadEvent{Head: head})
		}
	case 1: // done
		if i.doneHandler == nil {
			return
		}
		var e OutputManagerDoneEvent
		l := 0
		e.Serial = client.Uint32(data[l : l+4])
		l += 4
		i.doneHandler(e)
	case 2: // finished
		if i.finishedHandler == nil {
			return
		}
		i.finishedHandler(OutputManagerFinishedEvent{})
	}
}

// ---------------------------------------------------------------------------
// zwlr_output_head_v1
// ---------------------------------------------------------------------------

// OutputHead is an output device (monitor), enabled or not.
type OutputHead struct {
	client.BaseProxy
	nameHandler         OutputHeadNameHandlerFunc
	descriptionHandler  OutputHeadDescriptionHandlerFunc
	physicalSizeHandler OutputHeadPhysicalSizeHandlerFunc
	modeHandler         OutputHeadModeHandlerFunc
	enabledHandler      OutputHeadEnabledHandlerFunc
	currentModeHandler  OutputHeadCurrentModeHandlerFunc
	positionHandler     OutputHeadPositionHandlerFunc
	transformHandler    OutputHeadTransformHandlerFunc
	scaleHandler        OutputHeadScaleHandlerFunc
	finishedHandler     OutputHeadFinishedHandlerFunc
}

func NewOutputHead(ctx *client.Context) *OutputHead {
	h := &OutputHead{}
	// Note: registration with the specific ID assigned by the compositor
	// (via the manager's "head" event) happens through
	// ctx.RegisterWithID in OutputManager.Dispatch, not here - the ID for
	// a new_id-typed *event* argument is chosen by the compositor, not
	// the client, so we must not call ctx.Register (which would instead
	// allocate the next client-side ID).
	h.SetContext(ctx)
	return h
}

type OutputHeadNameEvent struct{ Name string }
type OutputHeadNameHandlerFunc func(OutputHeadNameEvent)

func (i *OutputHead) SetNameHandler(f OutputHeadNameHandlerFunc) { i.nameHandler = f }

type OutputHeadDescriptionEvent struct{ Description string }
type OutputHeadDescriptionHandlerFunc func(OutputHeadDescriptionEvent)

func (i *OutputHead) SetDescriptionHandler(f OutputHeadDescriptionHandlerFunc) {
	i.descriptionHandler = f
}

type OutputHeadPhysicalSizeEvent struct{ Width, Height int32 }
type OutputHeadPhysicalSizeHandlerFunc func(OutputHeadPhysicalSizeEvent)

func (i *OutputHead) SetPhysicalSizeHandler(f OutputHeadPhysicalSizeHandlerFunc) {
	i.physicalSizeHandler = f
}

type OutputHeadModeEvent struct{ Mode *OutputMode }
type OutputHeadModeHandlerFunc func(OutputHeadModeEvent)

func (i *OutputHead) SetModeHandler(f OutputHeadModeHandlerFunc) { i.modeHandler = f }

type OutputHeadEnabledEvent struct{ Enabled int32 }
type OutputHeadEnabledHandlerFunc func(OutputHeadEnabledEvent)

func (i *OutputHead) SetEnabledHandler(f OutputHeadEnabledHandlerFunc) { i.enabledHandler = f }

type OutputHeadCurrentModeEvent struct{ Mode *OutputMode }
type OutputHeadCurrentModeHandlerFunc func(OutputHeadCurrentModeEvent)

func (i *OutputHead) SetCurrentModeHandler(f OutputHeadCurrentModeHandlerFunc) {
	i.currentModeHandler = f
}

type OutputHeadPositionEvent struct{ X, Y int32 }
type OutputHeadPositionHandlerFunc func(OutputHeadPositionEvent)

func (i *OutputHead) SetPositionHandler(f OutputHeadPositionHandlerFunc) { i.positionHandler = f }

type OutputHeadTransformEvent struct{ Transform int32 }
type OutputHeadTransformHandlerFunc func(OutputHeadTransformEvent)

func (i *OutputHead) SetTransformHandler(f OutputHeadTransformHandlerFunc) {
	i.transformHandler = f
}

type OutputHeadScaleEvent struct{ Scale float64 }
type OutputHeadScaleHandlerFunc func(OutputHeadScaleEvent)

func (i *OutputHead) SetScaleHandler(f OutputHeadScaleHandlerFunc) { i.scaleHandler = f }

type OutputHeadFinishedEvent struct{}
type OutputHeadFinishedHandlerFunc func(OutputHeadFinishedEvent)

func (i *OutputHead) SetFinishedHandler(f OutputHeadFinishedHandlerFunc) { i.finishedHandler = f }

func (i *OutputHead) Dispatch(opcode uint32, fd int, data []byte) {
	switch opcode {
	case 0: // name
		if i.nameHandler == nil {
			return
		}
		l := 0
		n := client.PaddedLen(int(client.Uint32(data[l : l+4])))
		l += 4
		i.nameHandler(OutputHeadNameEvent{Name: client.String(data[l : l+n])})
	case 1: // description
		if i.descriptionHandler == nil {
			return
		}
		l := 0
		n := client.PaddedLen(int(client.Uint32(data[l : l+4])))
		l += 4
		i.descriptionHandler(OutputHeadDescriptionEvent{Description: client.String(data[l : l+n])})
	case 2: // physical_size
		if i.physicalSizeHandler == nil {
			return
		}
		var e OutputHeadPhysicalSizeEvent
		l := 0
		e.Width = int32(client.Uint32(data[l : l+4]))
		l += 4
		e.Height = int32(client.Uint32(data[l : l+4]))
		i.physicalSizeHandler(e)
	case 3: // mode (new_id)
		l := 0
		id := client.Uint32(data[l : l+4])
		mode := NewOutputMode(i.Context())
		i.Context().RegisterWithID(id, mode)
		if i.modeHandler != nil {
			i.modeHandler(OutputHeadModeEvent{Mode: mode})
		}
	case 4: // enabled
		if i.enabledHandler == nil {
			return
		}
		var e OutputHeadEnabledEvent
		e.Enabled = int32(client.Uint32(data[0:4]))
		i.enabledHandler(e)
	case 5: // current_mode (object)
		if i.currentModeHandler == nil {
			return
		}
		id := client.Uint32(data[0:4])
		proxy := i.Context().GetProxy(id)
		mode, _ := proxy.(*OutputMode)
		i.currentModeHandler(OutputHeadCurrentModeEvent{Mode: mode})
	case 6: // position
		if i.positionHandler == nil {
			return
		}
		var e OutputHeadPositionEvent
		l := 0
		e.X = int32(client.Uint32(data[l : l+4]))
		l += 4
		e.Y = int32(client.Uint32(data[l : l+4]))
		i.positionHandler(e)
	case 7: // transform
		if i.transformHandler == nil {
			return
		}
		var e OutputHeadTransformEvent
		e.Transform = int32(client.Uint32(data[0:4]))
		i.transformHandler(e)
	case 8: // scale (fixed)
		if i.scaleHandler == nil {
			return
		}
		var e OutputHeadScaleEvent
		e.Scale = client.Fixed(data[0:4])
		i.scaleHandler(e)
	case 9: // finished
		if i.finishedHandler != nil {
			i.finishedHandler(OutputHeadFinishedEvent{})
		}
	}
}

// ---------------------------------------------------------------------------
// zwlr_output_mode_v1
// ---------------------------------------------------------------------------

// OutputMode describes one supported mode (resolution + refresh rate) of a head.
type OutputMode struct {
	client.BaseProxy
	sizeHandler      OutputModeSizeHandlerFunc
	refreshHandler   OutputModeRefreshHandlerFunc
	preferredHandler OutputModePreferredHandlerFunc
	finishedHandler  OutputModeFinishedHandlerFunc
}

func NewOutputMode(ctx *client.Context) *OutputMode {
	m := &OutputMode{}
	m.SetContext(ctx)
	return m
}

type OutputModeSizeEvent struct{ Width, Height int32 }
type OutputModeSizeHandlerFunc func(OutputModeSizeEvent)

func (i *OutputMode) SetSizeHandler(f OutputModeSizeHandlerFunc) { i.sizeHandler = f }

type OutputModeRefreshEvent struct{ Refresh int32 } // mHz
type OutputModeRefreshHandlerFunc func(OutputModeRefreshEvent)

func (i *OutputMode) SetRefreshHandler(f OutputModeRefreshHandlerFunc) { i.refreshHandler = f }

type OutputModePreferredEvent struct{}
type OutputModePreferredHandlerFunc func(OutputModePreferredEvent)

func (i *OutputMode) SetPreferredHandler(f OutputModePreferredHandlerFunc) { i.preferredHandler = f }

type OutputModeFinishedEvent struct{}
type OutputModeFinishedHandlerFunc func(OutputModeFinishedEvent)

func (i *OutputMode) SetFinishedHandler(f OutputModeFinishedHandlerFunc) { i.finishedHandler = f }

func (i *OutputMode) Dispatch(opcode uint32, fd int, data []byte) {
	switch opcode {
	case 0: // size
		if i.sizeHandler == nil {
			return
		}
		var e OutputModeSizeEvent
		l := 0
		e.Width = int32(client.Uint32(data[l : l+4]))
		l += 4
		e.Height = int32(client.Uint32(data[l : l+4]))
		i.sizeHandler(e)
	case 1: // refresh
		if i.refreshHandler == nil {
			return
		}
		var e OutputModeRefreshEvent
		e.Refresh = int32(client.Uint32(data[0:4]))
		i.refreshHandler(e)
	case 2: // preferred
		if i.preferredHandler != nil {
			i.preferredHandler(OutputModePreferredEvent{})
		}
	case 3: // finished
		if i.finishedHandler != nil {
			i.finishedHandler(OutputModeFinishedEvent{})
		}
	}
}

// ---------------------------------------------------------------------------
// zwlr_output_configuration_v1
// ---------------------------------------------------------------------------

// OutputConfiguration describes a full output configuration the client wants
// to apply or test. Every currently-known head must be either enabled
// (EnableHead) or disabled (DisableHead) exactly once before Apply/Test.
type OutputConfiguration struct {
	client.BaseProxy
	succeededHandler OutputConfigurationSucceededHandlerFunc
	failedHandler    OutputConfigurationFailedHandlerFunc
	cancelledHandler OutputConfigurationCancelledHandlerFunc
}

func NewOutputConfiguration(ctx *client.Context) *OutputConfiguration {
	c := &OutputConfiguration{}
	ctx.Register(c)
	return c
}

// EnableHead enables a head and returns a configuration-head object used to
// set its mode/position/transform/scale.
func (i *OutputConfiguration) EnableHead(head *OutputHead) (*OutputConfigurationHead, error) {
	id := NewOutputConfigurationHead(i.Context())
	const opcode = 0
	const _reqBufLen = 8 + 4 + 4
	var _reqBuf [_reqBufLen]byte
	l := 0
	client.PutUint32(_reqBuf[l:4], i.ID())
	l += 4
	client.PutUint32(_reqBuf[l:l+4], uint32(_reqBufLen<<16|opcode&0x0000ffff))
	l += 4
	client.PutUint32(_reqBuf[l:l+4], id.ID())
	l += 4
	client.PutUint32(_reqBuf[l:l+4], head.ID())
	l += 4
	err := i.Context().WriteMsg(_reqBuf[:], nil)
	return id, err
}

// DisableHead disables a head.
func (i *OutputConfiguration) DisableHead(head *OutputHead) error {
	const opcode = 1
	const _reqBufLen = 8 + 4
	var _reqBuf [_reqBufLen]byte
	l := 0
	client.PutUint32(_reqBuf[l:4], i.ID())
	l += 4
	client.PutUint32(_reqBuf[l:l+4], uint32(_reqBufLen<<16|opcode&0x0000ffff))
	l += 4
	client.PutUint32(_reqBuf[l:l+4], head.ID())
	l += 4
	err := i.Context().WriteMsg(_reqBuf[:], nil)
	return err
}

// Apply applies the configuration. The compositor will reply with
// Succeeded, Failed or Cancelled.
func (i *OutputConfiguration) Apply() error {
	const opcode = 2
	const _reqBufLen = 8
	var _reqBuf [_reqBufLen]byte
	l := 0
	client.PutUint32(_reqBuf[l:4], i.ID())
	l += 4
	client.PutUint32(_reqBuf[l:l+4], uint32(_reqBufLen<<16|opcode&0x0000ffff))
	l += 4
	return i.Context().WriteMsg(_reqBuf[:], nil)
}

// Test validates the configuration without applying it.
func (i *OutputConfiguration) Test() error {
	const opcode = 3
	const _reqBufLen = 8
	var _reqBuf [_reqBufLen]byte
	l := 0
	client.PutUint32(_reqBuf[l:4], i.ID())
	l += 4
	client.PutUint32(_reqBuf[l:l+4], uint32(_reqBufLen<<16|opcode&0x0000ffff))
	l += 4
	return i.Context().WriteMsg(_reqBuf[:], nil)
}

// Destroy destroys the configuration object (destructor, opcode 4).
func (i *OutputConfiguration) Destroy() error {
	defer i.Context().Unregister(i)
	const opcode = 4
	const _reqBufLen = 8
	var _reqBuf [_reqBufLen]byte
	l := 0
	client.PutUint32(_reqBuf[l:4], i.ID())
	l += 4
	client.PutUint32(_reqBuf[l:l+4], uint32(_reqBufLen<<16|opcode&0x0000ffff))
	l += 4
	return i.Context().WriteMsg(_reqBuf[:], nil)
}

type OutputConfigurationSucceededEvent struct{}
type OutputConfigurationSucceededHandlerFunc func(OutputConfigurationSucceededEvent)

func (i *OutputConfiguration) SetSucceededHandler(f OutputConfigurationSucceededHandlerFunc) {
	i.succeededHandler = f
}

type OutputConfigurationFailedEvent struct{}
type OutputConfigurationFailedHandlerFunc func(OutputConfigurationFailedEvent)

func (i *OutputConfiguration) SetFailedHandler(f OutputConfigurationFailedHandlerFunc) {
	i.failedHandler = f
}

type OutputConfigurationCancelledEvent struct{}
type OutputConfigurationCancelledHandlerFunc func(OutputConfigurationCancelledEvent)

func (i *OutputConfiguration) SetCancelledHandler(f OutputConfigurationCancelledHandlerFunc) {
	i.cancelledHandler = f
}

func (i *OutputConfiguration) Dispatch(opcode uint32, fd int, data []byte) {
	switch opcode {
	case 0:
		if i.succeededHandler != nil {
			i.succeededHandler(OutputConfigurationSucceededEvent{})
		}
	case 1:
		if i.failedHandler != nil {
			i.failedHandler(OutputConfigurationFailedEvent{})
		}
	case 2:
		if i.cancelledHandler != nil {
			i.cancelledHandler(OutputConfigurationCancelledEvent{})
		}
	}
}

// ---------------------------------------------------------------------------
// zwlr_output_configuration_head_v1
// ---------------------------------------------------------------------------

// OutputConfigurationHead configures a single enabled head: mode (or custom
// mode), position, transform and scale. Request-only, no events.
type OutputConfigurationHead struct {
	client.BaseProxy
}

func NewOutputConfigurationHead(ctx *client.Context) *OutputConfigurationHead {
	h := &OutputConfigurationHead{}
	ctx.Register(h)
	return h
}

// SetMode assigns one of the head's advertised modes.
func (i *OutputConfigurationHead) SetMode(mode *OutputMode) error {
	const opcode = 0
	const _reqBufLen = 8 + 4
	var _reqBuf [_reqBufLen]byte
	l := 0
	client.PutUint32(_reqBuf[l:4], i.ID())
	l += 4
	client.PutUint32(_reqBuf[l:l+4], uint32(_reqBufLen<<16|opcode&0x0000ffff))
	l += 4
	client.PutUint32(_reqBuf[l:l+4], mode.ID())
	l += 4
	return i.Context().WriteMsg(_reqBuf[:], nil)
}

// SetCustomMode assigns an arbitrary width/height/refresh (mHz, 0 = unspecified).
func (i *OutputConfigurationHead) SetCustomMode(width, height, refresh int32) error {
	const opcode = 1
	const _reqBufLen = 8 + 4 + 4 + 4
	var _reqBuf [_reqBufLen]byte
	l := 0
	client.PutUint32(_reqBuf[l:4], i.ID())
	l += 4
	client.PutUint32(_reqBuf[l:l+4], uint32(_reqBufLen<<16|opcode&0x0000ffff))
	l += 4
	client.PutUint32(_reqBuf[l:l+4], uint32(width))
	l += 4
	client.PutUint32(_reqBuf[l:l+4], uint32(height))
	l += 4
	client.PutUint32(_reqBuf[l:l+4], uint32(refresh))
	l += 4
	return i.Context().WriteMsg(_reqBuf[:], nil)
}

// SetPosition sets the head's position in the global compositor space.
func (i *OutputConfigurationHead) SetPosition(x, y int32) error {
	const opcode = 2
	const _reqBufLen = 8 + 4 + 4
	var _reqBuf [_reqBufLen]byte
	l := 0
	client.PutUint32(_reqBuf[l:4], i.ID())
	l += 4
	client.PutUint32(_reqBuf[l:l+4], uint32(_reqBufLen<<16|opcode&0x0000ffff))
	l += 4
	client.PutUint32(_reqBuf[l:l+4], uint32(x))
	l += 4
	client.PutUint32(_reqBuf[l:l+4], uint32(y))
	l += 4
	return i.Context().WriteMsg(_reqBuf[:], nil)
}

// SetTransform sets the head's transform (wl_output.transform enum: 0=normal,
// 1..3=90/180/270, 4=flipped, 5..7=flipped-90/180/270).
func (i *OutputConfigurationHead) SetTransform(transform int32) error {
	const opcode = 3
	const _reqBufLen = 8 + 4
	var _reqBuf [_reqBufLen]byte
	l := 0
	client.PutUint32(_reqBuf[l:4], i.ID())
	l += 4
	client.PutUint32(_reqBuf[l:l+4], uint32(_reqBufLen<<16|opcode&0x0000ffff))
	l += 4
	client.PutUint32(_reqBuf[l:l+4], uint32(transform))
	l += 4
	return i.Context().WriteMsg(_reqBuf[:], nil)
}

// SetScale sets the head's scale factor.
func (i *OutputConfigurationHead) SetScale(scale float64) error {
	const opcode = 4
	const _reqBufLen = 8 + 4
	var _reqBuf [_reqBufLen]byte
	l := 0
	client.PutUint32(_reqBuf[l:4], i.ID())
	l += 4
	client.PutUint32(_reqBuf[l:l+4], uint32(_reqBufLen<<16|opcode&0x0000ffff))
	l += 4
	client.PutFixed(_reqBuf[l:l+4], scale)
	l += 4
	return i.Context().WriteMsg(_reqBuf[:], nil)
}
