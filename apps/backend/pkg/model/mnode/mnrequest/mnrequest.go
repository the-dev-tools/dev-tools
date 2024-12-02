package mnrequest

import "the-dev-tools/backend/pkg/idwrap"

type MNRequest struct {
	id        idwrap.IDWrap
	name      string
	exampleID idwrap.IDWrap
	next      idwrap.IDWrap
}
