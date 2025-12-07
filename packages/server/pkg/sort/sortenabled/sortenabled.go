//nolint:revive // exported
package sortenabled

type Enabled interface {
	IsEnabled() bool
}

// just get enabled etc...
func GetAllWithState[E Enabled](enables *[]E, state bool) {
	enablesTemp := *enables
	tempEnables := make([]E, 0, len(enablesTemp))
	for _, enablable := range enablesTemp {
		if enablable.IsEnabled() == state {
			tempEnables = append(tempEnables, enablable)
		}
	}
	*enables = tempEnables
}
