package jsonutil

import "reflect"

// ArrayExtras returns the elements of current that do not deep-equal any
// element of desired, preserving current's order. Used by doctor to surface
// live entries the manifest never declared (the append-accumulation signature).
func ArrayExtras(desired, current []any) []any {
	var extras []any
	for _, c := range current {
		found := false
		for _, d := range desired {
			if reflect.DeepEqual(c, d) {
				found = true
				break
			}
		}
		if !found {
			extras = append(extras, c)
		}
	}
	return extras
}
