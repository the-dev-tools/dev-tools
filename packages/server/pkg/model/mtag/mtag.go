//nolint:revive // exported
package mtag

import "github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"

type Color uint8

const (
	ColorSlate Color = iota
	ColorGreen
	ColorAmber
	ColorSky
	ColorPurple
	ColorRose
	ColorBlue
	ColorFuchsia
)

type Tag struct {
	ID          idwrap.IDWrap
	WorkspaceID idwrap.IDWrap
	Name        string
	Color       Color
}
